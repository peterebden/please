package worker

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
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
	"github.com/thought-machine/please/src/remote/fsclient"
	pb "github.com/thought-machine/please/src/remote/proto/remote"
	wpb "github.com/thought-machine/please/tools/mettle/proto/worker"
)

var log = logging.MustGetLogger("worker")

// Connect connects to the master and receives messages.
// It continues forever until the server disconnects.
func Connect(url, name, dir string, fileClient fsclient.Client) {
	conn := grpcutil.Dial(url)
	client := wpb.NewRemoteMasterClient(conn)
	stream, err := client.Work(context.Background())
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
	if err := w.cleanDir(); err != nil {
		log.Fatalf("Failed to clean workdir: %s", err)
	} else if err := os.Chdir(w.Dir); err != nil {
		log.Fatalf("Failed to chdir: %s", err)
	}
	go w.Run()

	// Send one response to the server, telling it that we're alive.
	if err := stream.Send(&wpb.WorkRequest{Name: name}); err != nil {
		log.Fatalf("Failed to send registration message: %s", err)
	}

	// Run heartbeats in the background forever
	go func() {
		stream, err := client.Heartbeat(context.Background())
		if err != nil {
			log.Fatalf("Failed to connect: %s", err)
		}
		for {
			if err := stream.Send(&wpb.HeartbeatRequest{Name: name}); err != nil {
				log.Fatalf("Failed to send heartbeat: %s", err)
			} else if _, err := stream.Recv(); err != nil {
				log.Fatalf("Stream terminated: %s", err)
			}
			time.Sleep(10 * time.Second)
		}
	}()

	// Now read responses until the server terminates.
	for {
		resp, err := stream.Recv()
		if err != nil {
			log.Fatalf("Stream terminated: %s", err)
		} else if resp.Shutdown {
			log.Warning("Got shutdown from server")
			break
		}
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
	Client    fsclient.Client
	Dir       string
	Requests  chan *pb.RemoteTaskRequest
	Responses chan *pb.RemoteTaskResponse
	hasher    *fs.PathHasher
}

// Run runs builds until its channel is exhausted.
func (w *worker) Run() {
	for req := range w.Requests {
		log.Notice("Received request to build %s", req.Target)
		resp := w.Build(req)
		if !resp.Success {
			// Something went wrong with the command.
			log.Warning("Finished building %s unsuccessfully: %s\nOutput: %s", req.Target, resp.Msg, resp.Output)
			w.Responses <- resp
			continue
		} else if req.Prompt && resp.Success {
			// We've built successfully, but the client needs to be prompted, so
			// we get an extra request / response pair
			resp.Done = false
			w.Responses <- resp
			req = <-w.Requests
		}
		files, err := w.collectOutputs(req)
		resp.Files = files
		if err != nil {
			log.Warning("Failed to collect output files: %s", err)
			resp.Success = false
			resp.Msg = err.Error()
		} else {
			log.Notice("Finished building %s successfully", req.Target)
		}
		w.Responses <- resp
	}
}

// Build runs a single build command.
func (w *worker) Build(req *pb.RemoteTaskRequest) *pb.RemoteTaskResponse {
	out, err := w.build(req)
	if err != nil {
		return &pb.RemoteTaskResponse{Msg: err.Error(), Output: out, Complete: true, Done: true}
	}
	return &pb.RemoteTaskResponse{Success: true, Output: out, Complete: true, Done: true}
}

// build runs the actual build command.
func (w *worker) build(req *pb.RemoteTaskRequest) ([]byte, error) {
	if err := w.cleanDir(); err != nil {
		return nil, err
	} else if err := w.setupFiles(req.Files); err != nil {
		return nil, err
	}
	// TODO(peterebden): Add support for the progress reporting pseudo-protocol.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-u", "-o", "pipefail", "-c", req.Command)
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

// cleanDir cleans the working directory and changes to it.
func (w *worker) cleanDir() error {
	if err := os.RemoveAll(w.Dir); err != nil {
		return err
	} else if err := os.MkdirAll(w.Dir, os.ModeDir|0755); err != nil {
		return err
	}
	return os.Chdir(w.Dir)
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
		log.Debug("setting up file %s", file.Filenames)
		rs, err := w.Client.Get(file.Filenames, file.Hash)
		if err != nil {
			log.Debug("error setting up file: %s", err)
			return err
		}
		for i, r := range rs {
			r := r
			filename := file.Filenames[i]
			g.Go(func() error {
				fullpath := path.Join(w.Dir, filename)
				if err := fs.EnsureDir(fullpath); err != nil {
					return err
				}
				f, err := os.Create(fullpath)
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = io.Copy(f, r)
				if err != nil && grpcutil.IsNotFound(err) {
					return fmt.Errorf("failed to download %s / %x from remote FS", filename, file.Hash)
				}
				return err
			})
		}
	}
	return g.Wait()
}

// collectOutputs collects all the output files from the build directory.
func (w *worker) collectOutputs(req *pb.RemoteTaskRequest) ([]*pb.Fileset, error) {
	if err := w.Client.PutRelative(req.Outputs, req.Hash, w.Dir, req.PackageName); err != nil {
		return nil, err
	}
	files := make([]*pb.Fileset, len(req.Outputs))
	for i, out := range req.Outputs {
		hash, err := w.hasher.UncachedHash(path.Join(w.Dir, out))
		if err != nil {
			return nil, err
		}
		files[i] = &pb.Fileset{
			Filenames: []string{out},
			Hash:      hash,
		}
	}
	return files, nil
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
