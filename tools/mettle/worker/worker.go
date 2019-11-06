// Package worker implements the worker side of Mettle.
package worker

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"gocloud.dev/pubsub"
	"google.golang.org/genproto/googleapis/longrunning"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/tools/mettle/common"
)

var log = logging.MustGetLogger("worker")

const timeout = 10 * time.Second

// RunForever runs the worker, receiving jobs until terminated.
func RunForever(requestQueue, responseQueue, storage, dir string) {
	if err := runForever(requestQueue, responseQueue, storage, dir); err != nil {
		log.Fatalf("%s", err)
	}
}

func runForever(requestQueue, responseQueue, storage, dir string) error {
	conn, err := grpc.Dial(storage,
		grpc.WithTimeout(timeout),
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(grpc_retry.WithMax(3))))
	if err != nil {
		return nil, nil, err
	}
	w := &worker{
		requests:  common.MustOpenSubscription(requestQueue),
		responses: common.MustOpenTopic(responseQueue),
		storage:   pb.NewContentAddressableStorageClient(conn),
		dir:       dir,
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM)
	go func() {
		log.Warning("Received signal %s, shutting down when ready...", <-ch)
		cancel()
		log.Fatalf("Received another signal %s, shutting down immediately", <-ch)
	}()
	for {
		if err := w.RunTask(ctx); err != nil {
			// If we get an error back here, we have failed to communicate with one of
			// our queues, so we are basically doomed and should stop.
			return fmt.Errorf("Failed to run task: %s", err)
		}
	}
}

type worker struct {
	requests  *pubsub.Subscription
	responses *pubsub.Topic
	storage   pb.ContentAddressableStorageClient
	dir       string
}

// RunTask runs a single task.
// Note that it only returns errors for reasons this service controls (i.e. queue comms),
// failures at actually running the task are communicated back on the responses queue.
func (w *worker) RunTask(ctx context.Context) error {
	log.Notice("Waiting for next task...")
	msg, err := w.requests.Receive(ctx)
	if err != nil {
		return err
	}
	// Mark message as consumed now. Alternatively we could not ack it until we
	// run the command, but we *probably* want to do that kind of retrying at a
	// higher level. TBD.
	msg.Ack()
	if err := w.runTask(msg.Body); err != nil {
		log.Warning("%s", err)
	}
}

// runTask does the actual running of a task.
func (w *worker) runTask(msg []byte) rpcstatus.Status {
	req := &pb.ExecuteRequest{}
	if err := proto.Unmarshal(msg, req); err != nil {
		return status(codes.BadRequest, "Badly serialised request: %s", err)
	} else if status := w.prepareDir(req); status != nil {
		return status
	}

}

// prepareDir prepares the directory for executing this request.
func (w *worker) prepareDir(req *pb.ExecuteRequest) *rpcstatus.Status {
	w.update(req.ActionDigest, pb.ExecutionStage_EXECUTING)

}

// update sends an update on the response channel
func (w *worker) update(digest *pb.Digest, stage pb.ExecutionStage_Value) error {
	body, _ := proto.Marshal(&pb.ExecuteOperationMetadata{
		Stage:        stage,
		ActionDigest: digest,
	})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return w.responses.Send(ctx, &pubsub.Message{Body: body})
}

func status(code codes.Code, msg string, args ...interface{}) *rpcstatus.Status {
	return &rpcstatus.Status{
		Code:    code,
		Message: fmt.Sprintf(msg, args...),
	}
}
