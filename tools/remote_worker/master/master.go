package master

import (
	"context"
	"io"
	"sync"

	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"grpcutil"
	pb "remote/proto/remote"
)

var log = logging.MustGetLogger("master")

type worker struct {
	Name   string
	Server pb.RemoteWorker_RemoteTaskServer
}

type master struct {
	workers []*worker
	busy    map[string]*worker
	mutex   sync.Mutex
}

func (m *master) GetTestWorker(ctx context.Context, req *pb.TestWorkerRequest) (*pb.TestWorkerResponse, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if len(m.workers) == 0 {
		return &pb.TestWorkerResponse{
			Error: "No workers available",
		}, nil
	}
	idx := len(m.workers) - 1
	w := m.workers[idx]
	m.workers = m.workers[0:idx]
	m.busy[w.Name] = w
	return &pb.TestWorkerResponse{
		Success: true,
		Url:     w.URL,
		Name:    w.Name,
	}, nil
}

func (m *master) ConnectWorker(srv pb.RemoteTestMaster_ConnectWorkerServer) error {
	msg, err := srv.Recv()
	if err != nil {
		return err
	} else if msg.Url == "" {
		if err := srv.Send(&pb.ConnectWorkerResponse{Error: "Must provide a URL"}); err != nil {
			return err
		}
		return io.EOF
	}
	// Name isn't compulsory.
	name := msg.Name
	if name == "" {
		name = msg.Url
	}
	m.register(msg)
	defer m.deregister(name)
	for {
		msg, err := srv.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		} else if msg.BuildComplete {
			m.free(name)
		}
	}
}

// register registers a newly joining worker.
func (m *master) register(msg *pb.ConnectWorkerRequest) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.workers = append(m.workers, &worker{
		Name: msg.Name,
		URL:  msg.Url,
	})
	log.Notice("Registered worker %s", msg.Name)
}

// deregister disconnects a previously registered worker.
func (m *master) deregister(name string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	// This guy might be busy; he shouldn't really be but it's possible.
	if _, present := m.busy[name]; present {
		delete(m.busy, name)
		return
	}
	// If not, find and remove it from the list.
	for i, w := range m.workers {
		if w.Name == name {
			m.workers = append(m.workers[:i], m.workers[i+1:]...)
			log.Notice("Deregistered worker %s", name)
			return
		}
	}
	log.Warning("Failed to deregister %s", name)
}

// free frees a worker once it has completed a build.
func (m *master) free(name string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if w, present := m.busy[name]; present {
		delete(m.busy, name)
		m.workers = append(m.workers, w)
	} else {
		log.Warning("Worker %s isn't in busy list", name)
	}
}

// Start starts serving the master gRPC server on the given port.
func Start(port int) {
	grpcutil.StartServer(createServer(), port)
}

// createServer breaks out some of the functionality of Start for testing.
func createServer() *grpc.Server {
	s := grpc.NewServer()
	m := &master{
		busy: map[string]*worker{},
	}
	pb.RegisterRemoteTestMasterServer(s, m)
	return s
}
