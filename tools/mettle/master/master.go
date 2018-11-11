package master

import (
	"io"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/op/go-logging.v1"

	"grpcutil"
	pb "src/remote/proto/remote"
	wpb "tools/mettle/proto/worker"
)

var log = logging.MustGetLogger("master")

type worker struct {
	Name   string
	stream wpb.RemoteMaster_WorkServer
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
	workers  []*worker
	mutex    sync.Mutex
	shutdown chan error
	retries  int
	pause    time.Duration
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
	w := &worker{
		Name:   req.Name,
		stream: stream,
	}
	m.mutex.Lock()
	m.workers = append(m.workers, w)
	m.mutex.Unlock()
	log.Notice("Registered worker %s", w.Name)

	// Now wait until the master is shutting down. All further communication
	// happens through the RemoteTask RPC.
	return <-m.shutdown
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
	stream.Send(&pb.RemoteTaskResponse{Msg: "Building remotely..."})
	if err := w.Send(req); err != nil {
		return err
	}
	// Stream any further messages from the client up to it.
	// This doesn't happen on most tasks but can on some (e.g. if there's a post-build function)
	go func() {
		for {
			if req, err := stream.Recv(); err == io.EOF {
				break
			} else if err != nil {
				log.Error("Error receiving message from client: %s", err)
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
		if resp, err := w.Recv(); err == io.EOF {
			break
		} else if err != nil {
			log.Error("Error receiving response from worker %s: %s", w.Name, err)
			return err
		} else if err := stream.Send(resp); err != nil {
			log.Error("Error sending response to client: %s", err)
			break
		}
	}
	return nil // Done!
}

func (m *master) acquireWorker() (*worker, error) {
	// If everything is busy, retry a number of times
	for i := 0; i < m.retries; i++ {
		m.mutex.Lock()
		if len(m.workers) > 0 {
			w := m.workers[len(m.workers)-1]
			m.workers = m.workers[:len(m.workers)-1]
			m.mutex.Unlock()
			return w, nil
		}
		m.mutex.Unlock()
		log.Warning("No workers available to service incoming request [attempt %d]", i+1)
		time.Sleep(m.pause)
	}
	log.Warning("No workers available to service request after %d tries, giving up", m.retries)
	return nil, status.Errorf(codes.ResourceExhausted, "No workers available")
}

func (m *master) releaseWorker(w *worker) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.workers = append(m.workers, w)
}

// Start starts serving the master gRPC server on the given port.
func Start(port int, retries int, pause time.Duration) {
	s, m, lis := createServer(port, retries, pause)
	s.Serve(lis)
	// Probably doesn't do much good, but this signals all the client goroutines to stop sleeping.
	m.StopAll()
}

// createServer breaks out some of the functionality of Start for testing.
func createServer(port int, retries int, pause time.Duration) (*grpc.Server, *master, net.Listener) {
	s := grpc.NewServer()
	m := &master{
		shutdown: make(chan error),
		retries:  retries,
		pause:    pause,
	}
	pb.RegisterRemoteWorkerServer(s, m)
	wpb.RegisterRemoteMasterServer(s, m)
	return s, m, grpcutil.SetupServer(s, port)
}
