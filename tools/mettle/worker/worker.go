package worker

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/fs"
	"github.com/thought-machine/please/src/grpcutil"
	pb "github.com/thought-machine/please/src/remote/proto/remote"
	wpb "github.com/thought-machine/please/tools/mettle/proto/worker"
)

var log = logging.MustGetLogger("worker")

// timeout defines a master timeout for processing streams.
// It is, by necessity, quite long, since we might have to deal with some long individual actions.
const timeout = 10 * time.Minute

// Connect connects to the master and receives messages.
// It continues forever until the server disconnects.
func Connect(url, name, dir string, fileClient FileClient) {
	conn := grpcutil.Dial(url)
	client := wpb.NewRemoteMasterClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	stream, err := client.Work(ctx)
	if err != nil {
		log.Fatalf("Failed to connect: %s", err)
	}

	// Start the worker
	root := path.Join(dir, "work")
	w := &worker{
		Client:    fileClient,
		Dir:       root,
		Requests:  make(chan *pb.RemoteTaskRequest),
		Responses: make(chan *pb.RemoteTaskResponse),
		hasher:    fs.NewPathHasher(root),
	}
	go w.Run()

	// Send one response to the server, telling it that we're alive.
	if err := stream.Send(&wpb.WorkRequest{Name: name}); err != nil {
		log.Fatalf("Failed to send registration message: %s", err)
	}

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
		if err := stream.Send(&wpb.WorkRequest{
			Response: <-w.Responses,
		}); err != nil {
			log.Error("Error sending response: %s", err)
		}
	}
	close(w.Requests)
	close(w.Responses)
}

type worker struct {
	Client    FileClient
	Dir       string
	Context   context.Context
	Requests  chan *pb.RemoteTaskRequest
	Responses chan *pb.RemoteTaskResponse
	hasher    *fs.PathHasher
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
			req = <-w.Requests
		}
		w.collectOutputs(req, resp)
		w.Responses <- resp
	}
}

// Build runs a single build command.
func (w *worker) Build(req *pb.RemoteTaskRequest) *pb.RemoteTaskResponse {
	out, err := w.build(req)
	if err != nil {
		return &pb.RemoteTaskResponse{Msg: err.Error(), Output: out, Complete: true}
	}
	return &pb.RemoteTaskResponse{Success: true, Output: out, Complete: true}
}

// build runs the actual build command.
func (w *worker) build(req *pb.RemoteTaskRequest) ([]byte, error) {
	if err := os.RemoveAll(w.Dir); err != nil {
		return nil, err
	} else if err := os.MkdirAll(w.Dir, os.ModeDir|0755); err != nil {
		return nil, err
	} else if err := w.setupFiles(req.Files); err != nil {
		return nil, err
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
	log.Debug("Running command %s\nENVIRONMENT:\n%s", req.Command, strings.Join(cmd.Env, "\n"))
	if err := cmd.Run(); err != nil {
		log.Debug("Command failed: %s", err)
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

// setupFiles sets up required files in the build directory.
func (w *worker) setupFiles(files []*pb.Fileset) error {
	// TODO(peterebden): We will need to limit parallelism here at some point.
	var g errgroup.Group
	for _, file := range files {
		rs, err := w.Client.Get(file.Filenames, file.Hash)
		if err != nil {
			return err
		}
		for i, r := range rs {
			r := r
			filename := file.Filenames[i]
			g.Go(func() error {
				f, err := os.Create(path.Join(w.Dir, filename))
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = io.Copy(f, r)
				return err
			})
		}
	}
	return g.Wait()
}

// collectOutputs collects all the output files from the build directory.
func (w *worker) collectOutputs(req *pb.RemoteTaskRequest, resp *pb.RemoteTaskResponse) {
	files := make([]*pb.Fileset, len(req.Outputs))
	var g errgroup.Group
	for i, out := range req.Outputs {
		out := out
		file := &pb.Fileset{Filenames: []string{out}}
		files[i] = file
		g.Go(func() error {
			filename := path.Join(w.Dir, out)
			hash, err := w.hasher.UncachedHash(filename)
			if err != nil {
				return err
			}
			f, err := os.Open(filename)
			if err != nil {
				return err
			}
			defer f.Close()
			return w.Client.Put(out, f, hash)
		})
	}
	if err := g.Wait(); err != nil {
		log.Debug("Failed to collect output files: %s", err)
		resp.Success = false
		resp.Msg = err.Error()
	} else {
		resp.Files = files
	}
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

// FileClient is a temporary interface for fetching / sending files to some futuristic
// storage system that doesn't exist yet.
type FileClient interface {
	// Get requests a set of files from the remote.
	// It returns a parallel list of readers for them, which are always of the same length
	// as the requested filenames (as long as there is no error). The caller should close them
	// all when done.
	Get(filenames []string, hash []byte) ([]io.ReadCloser, error)
	// Put dispatches a file to the remote
	Put(filename string, content io.Reader, hash []byte) error
}
