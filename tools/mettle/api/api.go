// Package api implements the remote execution API server.
package api

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"gocloud.dev/pubsub"
	"google.golang.org/genproto/googleapis/longrunning"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/tools/mettle/common"
)

var log = logging.MustGetLogger("api")

const timeout = 10 * time.Second

var totalRequests = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "mettle",
	Name:      "requests_total",
})
var cachedRequests = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "mettle",
	Name:      "cached_total",
})
var currentRequests = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "mettle",
	Name:      "requests_current",
})

func init() {
	prometheus.MustRegister(totalRequests)
	prometheus.MustRegister(cachedRequests)
	prometheus.MustRegister(currentRequests)
}

// ServeForever serves on the given port until terminated.
func ServeForever(port int, requestQueue, responseQueue, storage string) {
	if err := serveForever(port, requestQueue, responseQueue, storage); err != nil {
		log.Fatalf("%s", err)
	}
}

func serveForever(port int, requestQueue, responseQueue, storage string) error {
	conn, err := grpc.Dial(storage,
		grpc.WithTimeout(timeout),
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(grpc_retry.WithMax(3))))
	if err != nil {
		return err
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("Failed to listen on %s: %v", lis.Addr(), err)
	}
	srv := &server{
		requests:  common.MustOpenTopic(requestQueue),
		responses: common.MustOpenSubscription(responseQueue),
		storage:   pb.NewActionCacheClient(conn),
	}
	go srv.Receive()
	s := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_recovery.UnaryServerInterceptor(),
			grpc_prometheus.UnaryServerInterceptor,
		)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_recovery.StreamServerInterceptor(),
			grpc_prometheus.StreamServerInterceptor,
		)),
	)
	pb.RegisterExecutionServer(s, srv)
	grpc_prometheus.Register(s)
	return s.Serve(lis)
}

type server struct {
	requests  *pubsub.Topic
	responses *pubsub.Subscription
	storage   pb.ActionCacheClient
	jobs      map[string]*job
	mutex     sync.Mutex
}

func (s *server) Execute(req *pb.ExecuteRequest, stream pb.Execution_ExecuteServer) error {
	totalRequests.Inc()
	currentRequests.Inc()
	if !req.SkipCacheLookup {
		// Check cache upfront in case this has already happened.
		ctx, cancel := context.WithTimeout(stream.Context(), timeout)
		defer cancel()
		if ar, err := s.storage.GetActionResult(ctx, &pb.GetActionResultRequest{
			ActionDigest: req.ActionDigest,
			// Unfortunately we don't seem to have a sensible way of knowing here whether we
			// should inline stdout / stderr or not.
		}); err != nil {
			if status.Code(err) != codes.NotFound {
				log.Warning("Failed to contact action cache: %s", err)
			}
		} else {
			defer cachedRequests.Inc()
			defer currentRequests.Dec()
			metadata, _ := ptypes.MarshalAny(&pb.ExecuteOperationMetadata{
				Stage:        pb.ExecutionStage_COMPLETED,
				ActionDigest: req.ActionDigest,
			})
			response, _ := ptypes.MarshalAny(&pb.ExecuteResponse{
				Result:       ar,
				CachedResult: true,
				Status:       &rpcstatus.Status{Code: int32(codes.OK)},
			})
			return stream.Send(&longrunning.Operation{
				Name:     req.ActionDigest.Hash,
				Metadata: metadata,
				Done:     true,
				Result:   &longrunning.Operation_Response{Response: response},
			})
		}
	}
	ch := s.eventStream(req.ActionDigest, true)
	b, _ := proto.Marshal(req)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := s.requests.Send(ctx, &pubsub.Message{Body: b}); err != nil {
		log.Error("Failed to submit work to stream: %s", err)
		return err
	}
	return s.streamEvents(req.ActionDigest, ch, stream)
}

func (s *server) WaitExecution(req *pb.WaitExecutionRequest, stream pb.Execution_WaitExecutionServer) error {
	digest := &pb.Digest{Hash: req.Name}
	ch := s.eventStream(digest, false)
	if ch == nil {
		return status.Errorf(codes.NotFound, "No execution in progress for %s", req.Name)
	}
	return s.streamEvents(digest, ch, stream)
}

// streamEvents streams a series of events back to the client.
func (s *server) streamEvents(digest *pb.Digest, ch <-chan *longrunning.Operation, stream pb.Execution_ExecuteServer) error {
	for op := range ch {
		if err := stream.Send(op); err != nil {
			s.stopStream(digest, ch)
			return err
		}
	}
	return nil
}

// eventStream registers an event stream for a job.
func (s *server) eventStream(digest *pb.Digest, create bool) <-chan *longrunning.Operation {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	j, present := s.jobs[digest.Hash]
	if !present {
		if !create {
			return nil
		}
		any, _ := ptypes.MarshalAny(&pb.ExecuteOperationMetadata{
			Stage:        pb.ExecutionStage_QUEUED,
			ActionDigest: digest,
		})
		j = &job{Current: &longrunning.Operation{Metadata: any}}
		s.jobs[digest.Hash] = j
	}
	ch := make(chan *longrunning.Operation, 100)
	j.Streams = append(j.Streams, ch)
	return ch
}

// stopStream de-registers the given event stream for a job.
func (s *server) stopStream(digest *pb.Digest, ch <-chan *longrunning.Operation) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	job, present := s.jobs[digest.Hash]
	if !present {
		log.Error("stopStream for non-existant job %s", digest.Hash)
		return
	}
	for i, stream := range job.Streams {
		if stream == ch {
			job.Streams = append(job.Streams[:i], job.Streams[i+1:]...)
			break
		}
	}
}

// Receive runs forever, receiving responses from the queue.
func (s *server) Receive() {
	for {
		msg, err := s.responses.Receive(context.Background())
		if err != nil {
			log.Fatalf("Failed to receive message: %s", err)
		}
		s.process(msg)
		msg.Ack()
	}
}

// process processes a single message off the responses queue.
func (s *server) process(msg *pubsub.Message) {
	op := &longrunning.Operation{}
	metadata := &pb.ExecuteOperationMetadata{}
	if err := proto.Unmarshal(msg.Body, op); err != nil {
		log.Error("Failed to deserialise message: %s", err)
		return
	} else if err := ptypes.UnmarshalAny(op.Metadata, metadata); err != nil {
		log.Error("Failed to deserialise metadata: %s", err)
		return
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if j, present := s.jobs[metadata.ActionDigest.Hash]; !present {
		// This is legit, we are getting an update about a job someone else started.
		s.jobs[metadata.ActionDigest.Hash] = &job{Current: op}
	} else {
		j.Current = op
		for _, stream := range j.Streams {
			// Invoke this in a goroutine so we do not block.
			go func(ch chan<- *longrunning.Operation) {
				defer func() {
					recover() // Avoid any chance of panicking from a 'send on closed channel'
				}()
				ch <- op
				if op.Done {
					close(ch)
				}
			}(stream)
		}
		if op.Done {
			delete(s.jobs, metadata.ActionDigest.Hash)
			currentRequests.Dec()
		}
	}
}

// A job represents a single execution request.
type job struct {
	Streams []chan *longrunning.Operation
	Current *longrunning.Operation
}
