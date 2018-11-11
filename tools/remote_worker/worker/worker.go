package worker

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"build"
	"cli"
	"core"
	"fs"
	"grpcutil"
	pb "src/remote/proto/remote"
	wpb "tools/remote_worker/proto/worker"
)

var log = logging.MustGetLogger("worker")

// timeout defines a master timeout for processing streams.
// It is, by necessity, quite long, since we might have to deal with some long individual actions.
const timeout = 10 * time.Minute

// Connect connects to the master and receives messages.
// It continues forever until the server disconnects.
func Connect(url, name string, dir string) {
	conn, err := grpcutil.Dial(url)
	if err != nil {
		log.Fatalf("Failed to dial server: %s", err)
	}
	client := pb.NewRemoteMasterClient(conn)
	ctx, cancel := context.WithTimeout(timeout)
	defer cancel()
	stream, err := client.Work(ctx)
	if err != nil {
		log.Fatalf("Failed to connect: %s", err)
	} else if err := stream.Send(&wpb.WorkRequest{Name: name}); err != nil {
		log.Fatalf("Failed to connect: %s", err)
	}

	// Start the worker
	w := &worker{
		Dir:       path.Join(dir, "work"),
		Requests:  make(chan *pb.RemoteTaskRequest),
		Responses: make(chan *pb.RemoteTaskResponse),
	}
	go w.Run()

	// Now read responses until the server terminates.
	for {
		resp, err := stream.Recv()
		if err != nil {
			log.Fatalf("Stream terminated: %s", err)
		} else if resp.Shutdown {
			log.Warning("Got shutdown from server")
			break
		}
		w.Context = stream.Context()
		w.Requests <- resp.Request
		if err := stream.Send(&pb.WorkRequest{
			Response: <-resp.Responses,
		}); err != nil {
			log.Error("Error sending response: %s", err)
		}
	}
	close(w.Requests)
	close(w.Responses)
}

type worker struct {
	Dir       string
	Context   context.Context
	Requests  chan *pb.RemoteTaskRequest
	Responses chan *pb.RemoteTaskResponse
}

// Run runs builds until its channel is exhausted.
func (w *worker) Run() {
	for req := range w.Requests {
		resp := w.Build(req)
		if !resp.Success {
			// Something went wrong with the command.
			w.Responses <- resp
			continue
		} else if req.Prompt && resp.Success {
			// We've built successfully, but the client needs to be prompted, so
			// we get an extra request / response pair
			w.Responses <- resp
			req <- w.Requests
		}
		w.CollectOutputs(req, resp)
		w.Responses <- resp
	}
}

// Build runs a single build command.
func (w *worker) Build(req *pb.RemoteTaskRequest) *pb.RemoteTaskResponse {
	if out, err := w.build(req); err != nil {
		return &pb.RemoteTaskResponse{Msg: err.Error(), Output: out}
	}
	return &pb.RemoteTaskResponse{Success: true, Output: out}
}

// build runs the actual build command.
func (w *worker) build(req *pb.RemoteTestRequest) ([]byte, error) {
	if err := os.RemoveAll(w.Dir); err != nil {
		return err
	} else if err := os.MkdirAll(w.Dir, os.ModeDir|0755); err != nil {
		return err
	} else if err := w.setupFiles(req.Files); err != nil {
		return err
	}
	// TODO(peterebden): Add support for the progress reporting pseudo-protocol.
	cmd := exec.CommandContext(w.Context, "bash", "-u", "-o", "pipefail", "-c", req.Command)
	cmd.Env = w.replaceEnv(req.Env)
	// We need to record both stdout (sent back on success for post-build functions) and combined
	// stdout / stderr (sent back on failure so the user can see everything).
	var out bytes.Buffer
	var outerr safeBuffer
	cmd.Stdout = io.MultiWriter(&out, &outerr)
	cmd.Stderr = &outerr
	log.Notice("Running command %s\nENVIRONMENT:\n%s\n%s", req.Command, strings.Join(cmd.Env, "\n"), cmd)
	if err := cmd.Run(); err != nil {
		return outerr.Bytes(), err
	}
	return out.Bytes(), nil
}

// replaceEnv replaces placeholders in environment variables.
func (w *worker) replaceEnv(env []string) []string {
	for i, e := range env {
		env[i] = strings.Replace(e, "${TMP_DIR}", w.Dir, 1)
	}
	return env
}

// safeBuffer is cloned from core.
// TODO(peterebden): find somewhere sensible to put this.
type safeBuffer struct {
	sync.Mutex
	buf bytes.Buffer
}

func (sb *safeBuffer) Write(b []byte) (int, error) {
	sb.Lock()
	defer sb.Unlock()
	return sb.buf.Write(b)
}

func (sb *safeBuffer) Bytes() []byte {
	return sb.buf.Bytes()
}
