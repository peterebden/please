package master

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/grpcutil"
	pb "github.com/thought-machine/please/src/remote/proto/remote"
	wpb "github.com/thought-machine/please/tools/mettle/proto/worker"
)

var log = logging.MustGetLogger("master")

var numWorkers = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "mettle_workers",
	Help: "Current number of total registered workers.",
})
var availableWorkers = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "mettle_available_workers",
	Help: "Current number of total available workers.",
})

func init() {
	prometheus.MustRegister(numWorkers)
	prometheus.MustRegister(availableWorkers)
}

type worker struct {
	Name      string
	Heartbeat time.Time
	stream    wpb.RemoteMaster_WorkServer
}

func (w *worker) Send(req *pb.RemoteTaskRequest) error {
	return w.stream.Send(&wpb.WorkResponse{Request: req})
}

func (w *worker) Recv() (*pb.RemoteTaskResponse, error) {
	resp, err := w.stream.Recv()
	if err != nil {
		return nil, err
	}
	return resp.Response, nil
}

type master struct {
	workers       []*worker
	workersByName map[string]*worker
	mutex         sync.Mutex
	shutdown      chan error
	retries       int
	pause         time.Duration
}

// StopAll releases all currently waiting server goroutines.
func (m *master) StopAll() {
	// *Theoretically* at this point all workers should be available, because all
	// RPCs should have been terminated before the server shut down.
	for i := 0; i < len(m.workers); i++ {
		m.shutdown <- status.Errorf(codes.OK, "Server shutting down")
	}
}

// Work implements the internal RPC for communication between us and the worker instances.
func (m *master) Work(stream wpb.RemoteMaster_WorkServer) error {
	// First message registers the worker
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	w := m.createWorker(req.Name)
	w.stream = stream
	// Now wait until the master is shutting down. All further communication
	// happens through the RemoteTask RPC.
	return <-m.shutdown
}

// Heartbeat implements the internal RPC for making sure clients are still alive.
func (m *master) Heartbeat(stream wpb.RemoteMaster_HeartbeatServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	w := m.createWorker(req.Name)
	for {
		if _, err := stream.Recv(); err != nil {
			log.Warning("Error heartbeating worker %s: %s", w.Name, err)
			m.deleteWorker(w)
			return err
		}
		w.Heartbeat = time.Now()
	}
}

// RemoteTask implements the external RPC, for handling requests from clients.
func (m *master) RemoteTask(stream pb.RemoteWorker_RemoteTaskServer) error {
	// Receive the build request
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	// Grab a worker
	w, err := m.acquireWorker()
	if err != nil {
		return err
	}
	defer m.releaseWorker(w)
	// Send it the task
	log.Notice("Assigning worker %s to build %s", w.Name, req.Target)
	stream.Send(&pb.RemoteTaskResponse{Success: true, Msg: "Building remotely..."})
	if err := w.Send(req); err != nil {
		return err
	}
	// Stream any further messages from the client up to it.
	// This doesn't happen on most tasks but can on some (e.g. if there's a post-build function)
	done := false
	go func() {
		for {
			if req, err := stream.Recv(); err == io.EOF {
				break
			} else if err != nil {
				if !done {
					log.Error("Error receiving message from client: %s", err)
				}
				break
			} else if err := w.Send(req); err != nil {
				log.Error("Error forwarding client message to worker: %s", err)
				stream.Send(&pb.RemoteTaskResponse{
					Complete: true,
					Msg:      err.Error(),
				})
			}
		}
	}()
	// Forward all the messages from the worker back down to the client.
	for {
		if resp, err := w.Recv(); err != nil {
			log.Error("Error receiving response from worker %s: %s", w.Name, err)
			return err
		} else if err := stream.Send(resp); err != nil {
			log.Error("Error sending response to client: %s", err)
			break
		} else if resp.Done {
			break
		}
	}
	done = true
	log.Notice("Finishing remote task %s and freeing %s", req.Target, w.Name)
	return nil // Done!
}

func (m *master) acquireWorker() (*worker, error) {
	// If everything is busy, retry a number of times
	for i := 0; i < m.retries; i++ {
		m.mutex.Lock()
		if len(m.workers) > 0 {
			w := m.workers[len(m.workers)-1]
			m.workers = m.workers[:len(m.workers)-1]
			availableWorkers.Dec()
			m.mutex.Unlock()
			return w, nil
		}
		m.mutex.Unlock()
		if i < m.retries-1 {
			log.Warning("No workers available to service incoming request [attempt %d]", i+1)
			time.Sleep(m.pause)
		}
	}
	log.Warning("No workers available to service request after %d tries, giving up", m.retries)
	return nil, status.Errorf(codes.ResourceExhausted, "No workers available")
}

func (m *master) releaseWorker(w *worker) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	// Check if this is in the name map - if not it has probably been terminated already.
	if _, present := m.workersByName[w.Name]; present {
		m.workers = append(m.workers, w)
		availableWorkers.Inc()
	}
}

// createWorker creates a new worker or returns an existing one by that name.
func (m *master) createWorker(name string) *worker {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if w, present := m.workersByName[name]; present {
		return w
	}
	w := &worker{Name: name}
	m.workersByName[name] = w
	m.workers = append(m.workers, w)
	numWorkers.Inc()
	availableWorkers.Inc()
	log.Notice("Added worker %s, now have %d total", name, len(m.workersByName))
	return w
}

func (m *master) deleteWorker(w *worker) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.workersByName, w.Name)
	for i, w2 := range m.workers {
		if w2 == w {
			m.workers = append(m.workers[:i], m.workers[i+1:]...)
			availableWorkers.Dec()
			break
		}
	}
	numWorkers.Dec()
	log.Notice("Removed worker %s, now have %d total", w.Name, len(m.workersByName))
}

// Start starts serving the master gRPC server on the given port.
func Start(port int, retries int, pause time.Duration) {
	s, m, lis := createServer(port, retries, pause)
	grpcutil.AddCleanup(m.StopAll)
	s.Serve(lis)
}

// createServer breaks out some of the functionality of Start for testing.
func createServer(port int, retries int, pause time.Duration) (*grpc.Server, *master, net.Listener) {
	s := grpc.NewServer()
	m := &master{
		shutdown:      make(chan error),
		retries:       retries,
		pause:         pause,
		workersByName: map[string]*worker{},
	}
	pb.RegisterRemoteWorkerServer(s, m)
	wpb.RegisterRemoteMasterServer(s, m)
	return s, m, grpcutil.SetupServer(s, port)
}
