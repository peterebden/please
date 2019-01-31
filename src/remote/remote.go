// +build !bootstrap

// Package remote is responsible for communication with remote build servers.
// Some of the nomenclature can be a little confusing since we use "remote" in other contexts
// (e.g. local worker servers that are "remote" to this process). Eventually we might clean
// that all up a bit to be a bit more consistent.
package remote

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/grpcutil"
	"github.com/thought-machine/please/src/remote/fsclient"
	pb "github.com/thought-machine/please/src/remote/proto/remote"
)

var log = logging.MustGetLogger("remote")

var remoteClient pb.RemoteWorkerClient
var remoteFSClient fsclient.Client
var remoteClientOnce sync.Once

const dialTimeout = 10 * time.Second

// Build causes a target to be built on a remote worker.
//
// N.B. It does *not* necessarily cause outputs to appear locally.
func Build(tid int, state *core.BuildState, target *core.BuildTarget, hash []byte) error {
	state.LogBuildResult(tid, target.Label, core.TargetBuilding, "Dispatching to remote worker...")
	remoteClientOnce.Do(func() { initClient(state) })

	timeout := core.TimeoutOrDefault(target.BuildTimeout, state.Config.Build.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := remoteClient.RemoteTask(ctx)
	if err != nil {
		return err
	}
	defer client.CloseSend()
	// Send initial request
	sources, localfiles, err := convertSources(state, target)
	if err != nil {
		return err
	} else if len(localfiles) > 0 {
		// There were some local sources, we have to make sure the remote has them.
		// These are special-cased because we never actually build them (whereas for anything in
		// plz-out we assume something must have built it already).
		for _, localfile := range localfiles {
			log.Debug("Storing local source with remote FS: %s", localfile.Filenames)
			remoteFSClient.PutRelative(localfile.Filenames, localfile.Hash, "", "")
		}
	}
	prompt := target.PostBuildFunction != nil
	if err := client.Send(&pb.RemoteTaskRequest{
		Target:      target.Label.String(),
		PackageName: target.Label.PackageName,
		Command:     target.GetCommand(state),
		Hash:        hash,
		Outputs:     target.Outputs(),
		Prompt:      prompt,
		Env:         core.StampedBuildEnvironment(state, target, hash, "${TMP_DIR}/"),
		Files:       sources,
	}); err != nil {
		return err
	}
	// Now read responses
	for {
		resp, err := client.Recv()
		if err != nil {
			return err // including EOF, we should usually break before that.
		} else if !resp.Success {
			return fmt.Errorf("%s", resp.Msg)
		} else if resp.Complete {
			if !prompt {
				state.LogBuildResult(tid, target.Label, core.TargetBuiltRemotely, "Built remotely")
				recordHashes(state, target, resp)
				break // We are done.
			}
			// Target has a post-build function to be run, which we have to do.
			if err := state.Parser.RunPostBuildFunction(tid, state, target, string(resp.Output)); err != nil {
				return err
			}
			// Communicate the new state of the target back again.
			if err := client.Send(&pb.RemoteTaskRequest{Outputs: target.Outputs()}); err != nil {
				return err
			}
			prompt = false // we won't do this again
		} else {
			// Just a progress message.
			state.LogBuildResult(tid, target.Label, core.TargetBuilding, resp.Msg)
		}
	}
	return nil
}

// BuildFilegroup is a special case of the above for "building" a filegroup (which doesn't
// require an actual "build" action, but may require some files to be recorded remotely).
func BuildFilegroup(state *core.BuildState, target *core.BuildTarget) error {
	dir := target.ShortOutDir()
	for _, src := range target.FilegroupPaths(state, true) {
		if out := path.Join(dir, src.Tmp); out != src.Src {
			panic("BuildFilegroup: not implemented")
		}
	}
	return nil
}

// recordHashes records a set of hashes once we've built a target.
func recordHashes(state *core.BuildState, target *core.BuildTarget, resp *pb.RemoteTaskResponse) {
	outDir := target.OutDir()
	for _, file := range resp.Files {
		for _, filename := range file.Filenames {
			state.PathHasher.SetHash(path.Join(outDir, filename), file.Hash)
		}
	}
}

// initClient sets up the remote client
func initClient(state *core.BuildState) {
	// TODO(peterebden): TLS, as usual...
	conn := grpcutil.Dial(state.Config.Build.RemoteURL)
	remoteClient = pb.NewRemoteWorkerClient(conn)
	remoteFSClient = fsclient.Get(state.Config.Build.RemoteFSURL)
}

func convertSources(state *core.BuildState, target *core.BuildTarget) ([]*pb.Fileset, []*pb.Fileset, error) {
	allfiles := []*pb.Fileset{}
	localfiles := []*pb.Fileset{}
	for source := range core.IterSources(state.Graph, target) {
		hash, err := state.PathHasher.Hash(source.Src, false)
		if err != nil {
			return nil, nil, err
		}
		fs := &pb.Fileset{Hash: hash, Filenames: []string{source.Src}}
		if strings.HasPrefix(source.Src, core.OutDir) {
			fs.Filenames[0] = trimDirPrefix(trimDirPrefix(source.Src, core.GenDir), core.BinDir)
		} else {
			localfiles = append(localfiles, fs)
		}
		allfiles = append(allfiles, fs)
	}
	return allfiles, localfiles, nil
}

func trimDirPrefix(s, prefix string) string {
	if strings.HasPrefix(s, prefix) {
		return strings.TrimLeft(strings.TrimPrefix(s, prefix), "/")
	}
	return s
}

// IsRetryableLocally returns true if an error indicates a failure that can be usefully retried locally.
func IsRetryableLocally(err error) bool {
	return grpcutil.IsResourceExhausted(err) // if the remote was out of workers
}
