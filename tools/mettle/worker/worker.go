// Package worker implements the worker side of Mettle.
package worker

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/bazelbuild/remote-apis-sdks/go/client"
	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"gocloud.dev/pubsub"
	"google.golang.org/genproto/googleapis/longrunning"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/tools/mettle/common"
)

var log = logging.MustGetLogger("worker")

const timeout = 30 * time.Second

// RunForever runs the worker, receiving jobs until terminated.
func RunForever(requestQueue, responseQueue, storage, dir string) {
	if err := runForever(requestQueue, responseQueue, storage, dir); err != nil {
		log.Fatalf("%s", err)
	}
}

func runForever(requestQueue, responseQueue, storage, dir string) error {
	client, err := client.NewClient(context.Background(), "", client.DialParams{Service: storage})
	if err != nil {
		return err
	}
	w := &worker{
		requests:  common.MustOpenSubscription(requestQueue),
		responses: common.MustOpenTopic(responseQueue),
		storage:   pb.NewContentAddressableStorageClient(client.CASConnection),
		client:    client,
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
	requests     *pubsub.Subscription
	responses    *pubsub.Topic
	storage      pb.ContentAddressableStorageClient
	client       *client.Client
	dir          string
	actionDigest *pb.Digest
	metadata     *pb.ExecutedActionMetadata
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
	response := w.runTask(msg.Body)
	return w.update(pb.ExecutionStage_COMPLETED, response)
}

// runTask does the actual running of a task.
func (w *worker) runTask(msg []byte) *pb.ExecuteResponse {
	w.metadata = &pb.ExecutedActionMetadata{WorkerStartTimestamp: ptypes.TimestampNow()}
	req, action, command, status := w.readRequest(msg)
	if status != nil {
		return &pb.ExecuteResponse{
			Result: &pb.ActionResult{},
			Status: status,
		}
	}
	w.actionDigest = req.ActionDigest
	if status := w.prepareDir(action); status != nil {
		return &pb.ExecuteResponse{
			Result: &pb.ActionResult{},
			Status: status,
		}
	}
	return w.execute(action, command)
}

// readRequest unmarshals the original execution request.
func (w *worker) readRequest(msg []byte) (*pb.ExecuteRequest, *pb.Action, *pb.Command, *rpcstatus.Status) {
	req := &pb.ExecuteRequest{}
	action := &pb.Action{}
	command := &pb.Command{}
	if err := proto.Unmarshal(msg, req); err != nil {
		return nil, nil, nil, status(codes.FailedPrecondition, "Badly serialised request: %s", err)
	} else if err := c.readBlobToProto(req.ActionDigest, action); err != nil {
		return nil, nil, nil, status(codes.FailedPrecondition, "Invalid action digest: %s", err)
	} else if err := c.readBlobToProto(action.CommandDigest, command); err != nil {
		return nil, nil, nil, status(codes.FailedPrecondition, "Invalid command digest: %s", err)
	}
	return req, action, command, nil
}

// prepareDir prepares the directory for executing this request.
func (w *worker) prepareDir(action *pb.Action) *rpcstatus.Status {
	w.update(pb.ExecutionStage_EXECUTING, nil)
	w.metadata.InputFetchStartTimestamp = ptypes.TimestampNow()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	dirs, err := w.client.GetDirectoryTree(ctx, action.InputRootDigest)
	if err != nil {
		return status(codes.InvalidPrecondition, "Invalid input root: %s", err)
	}
	for _, dir := range dirs {
		dirname := path.Join(w.dir, dir.Name)
		for _, file := range dir.Files {
			filename := path.Join(dirname, file.Name)
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if _, err := w.client.ReadBlobToFile(ctx, file.Digest, filename); err != nil {
				return status(codes.InternalError, "Failed to read blob: %s", err)
			} else if file.IsExecutable {
				if err := os.Chmod(filename, 0555); err != nil {
					return status(codes.InternalError, "Failed to make file executable: %s", err)
				}
			}
		}
		for _, sym := range dir.Symlinks {
			if err := os.Symlink(sym.Target, path.Join(dirname, sym.Name)); err != nil {
				return status(codes.InternalError("Failed to create symlink: %s", err))
			}
		}
	}
	w.metadata.InputFetchCompletedTimestamp = ptypes.TimestampNow()
	return nil
}

// execute runs the actual commands once the inputs are prepared.
func (w *worker) execute(action *pb.Action, command *pb.Command) *pb.ExecuteResponse {
	w.ExecutionStartTimestamp = ptypes.TimestampNow()
	duration, _ := ptypes.Duration(action.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	cmd := exec.CommandContext(ctx, command.Arguments[0], command.Arguments[1:]...)
	cmd.Dir = path.Join(w.dir, command.WorkingDirectory)
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &bytes.Buffer{}
	cmd.Env = make([]string, len(command.EnvironmentVariables))
	for i, v := range command.EnvironmentVariables {
		cmd.Env[i] = v.Name + "=" + v.Value
	}
	err := cmd.Run()
	w.ExecutionCompletedTimestamp = ptypes.TimestampNow()
	w.OutputUploadStartTimestamp = ptypes.TimestampNow()
	// Regardless of the result, upload stdout / stderr.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	stdoutDigest, _ := w.client.WriteBlob(ctx, cmd.Stdout.Bytes())
	stderrDigest, _ := w.client.WriteBlob(ctx, cmd.Stderr.Bytes())
	ar := &pb.ActionResult{
		ExitCode:     int32(cmd.ProcessState.ExitCode()),
		StdoutDigest: stdoutDigest,
		StderrDigest: stderrDigest,
		Metadata:     w.metadata,
	}
	if err != nil {
		return &pb.ExecuteResponse{
			Status:       status(codes.Unknown, "Execution failed: %s", err),
			ActionResult: ar,
		}
	}
	// TODO(peterebden): Convert this to use the new output scheme once that is available.
	for _, out := range append(command.OutputFiles, command.OutputDirectories...) {
		if err := w.collectOutput(ar, out); err != nil {
			return &pb.ExecuteResponse{
				Status:       status(codes.Unknown, "Failed to collect output %s: %s", out, err),
				ActionResult: ar,
			}
		}
	}
	w.metadata.OutputUploadCompletedTimestamp = ptypes.TimestampNow()
	return &pb.ExecuteResponse{
		Status:       &rpcstatus.Status{Code: codes.OK},
		ActionResult: ar,
	}
}

// collectOutput collects a single output and adds it to the given ActionResult.
func (w *worker) collectOutput(ar *pb.ActionResult, output string) error {
	filename := path.Join(w.dir, output)
	if info, err := os.Lstat(filename); err != nil {
		return err
	} else if mode := info.Mode(); info.IsDir() {
		dir, _, children, err := w.collectDir(output)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		digest, err := w.client.WriteProto(ctx, &pb.Tree{
			Root:     dir,
			Children: children,
		})
		if err != nil {
			return err
		}
		ar.OutputDirectories = append(ar.OutputDirectories, &pb.OutputDirectory{
			Path:       output,
			TreeDigest: digest,
		})
	} else if mode&os.ModeSymlink == os.ModeSymlink {
		target, err := os.Readlink(filename)
		if err != nil {
			return err
		}
		ar.OutputFileSymlinks = append(ar.OutputFileSymlinks, &pb.OutputSymlink{
			Path:   output,
			Target: target,
		})
	} else { // regular file
		digest, err := w.collectFile(output)
		if err != nil {
			return err
		}
		ar.OutputFiles = append(ar.OutputFiles, &pb.OutputFile{
			Path:         output,
			Digest:       digest,
			IsExecutable: mode&0100 != 0,
		})
	}
	return nil
}

// collectDir collects a directory and, recursively, all its descendants.
func (w *worker) collectDir(dirname string) (*pb.Directory, *pb.Digest, []*pb.Directory, error) {
	d := &pb.Directory{}
	var children []*pb.Directory
	entries, err := ioutil.ReadDir(dirname)
	if err != nil {
		return nil, nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		filename := path.Join(dirname, name)
		if entry.IsDir() {
			dir, digest, children, err := w.collectDir(filename)
			if err != nil {
				return nil, nil, err
			}
			d.Directories = append(d.Directories, &pb.DirectoryNode{
				Name:   name,
				Digest: digest,
			})
			children = append(children, dir)
		} else if mode := entry.Mode(); mode&os.ModeSymlink != 0 {
			target, err := os.Readlink(filename)
			if err != nil {
				return nil, nil, err
			}
			d.Symlinks = append(d.Symlinks, &pb.SymlinkNode{
				Name:   name,
				Target: target,
			})
		} else {
			digest, err := w.collectFile(filename)
			if err != nil {
				return nil, nil, err
			}
			d.Files = append(d.Files, &pb.FileNode{
				Name:         name,
				Digest:       digest,
				IsExecutable: mode&0100 != 0,
			})
		}
	}
	// now digest and upload this directory
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	digest, err := w.client.WriteProto(ctx, d)
	return d, digest, children, err
}

// collectFile collects a single file.
func (w *worker) collectFile(filename string) (*pb.Digest, error) {
	// TODO(peterebden): This is a bit crap (reading the whole thing into memory)
	//                   but the sdk library doesn't offer much of help here.
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return w.client.WriteBlob(ctx, b)
}

// update sends an update on the response channel
func (w *worker) update(stage pb.ExecutionStage_Value, response *pb.ExecuteResponse) error {
	any, _ := ptypes.MarshalAny(&pb.ExecuteOperationMetadata{
		Stage:        stage,
		ActionDigest: w.actionDigest,
	})
	op := &longrunning.Operation{Metadata: any}
	if response != nil {
		any, _ = ptypes.MarshalAny(response)
		op.Result = &longrunning.Operation_Response{Response: any}
	}
	body, _ := proto.Marshal()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return w.responses.Send(ctx, &pubsub.Message{Body: body})
}

// readBlobToProto reads an entire blob and deserialises it into a message.
func (c *Client) readBlobToProto(digest *pb.Digest, msg proto.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	b, err := c.client.ReadBlob(ctx, digest)
	if err != nil {
		return err
	}
	return proto.Unmarshal(b, msg)
}

func status(code codes.Code, msg string, args ...interface{}) *rpcstatus.Status {
	return &rpcstatus.Status{
		Code:    code,
		Message: fmt.Sprintf(msg, args...),
	}
}
