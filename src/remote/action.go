package remote

import (
	"runtime"
	"sort"
	"strings"

	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/golang/protobuf/ptypes"

	"github.com/thought-machine/please/src/core"
)

// uploadAction uploads a build action for a target and returns its digest.
func (c *Client) uploadAction(target *core.BuildTarget, stamp []byte) (digest *pb.Digest, err error) {
	err = c.uploadBlobs(func(ch chan<- *blob) error {
		defer close(ch)
		inputRoot, err := c.buildInputRoot(target, true)
		if err != nil {
			return err
		}
		inputRootDigest, inputRootMsg := digestMessageContents(inputRoot)
		ch <- &blob{Data: inputRootMsg, Digest: *inputRootDigest}
		commandDigest, commandMsg := digestMessageContents(c.buildCommand(target, stamp))
		ch <- &blob{Data: commandMsg, Digest: *commandDigest}
		action := &pb.Action{
			CommandDigest:   commandDigest,
			InputRootDigest: inputRootDigest,
			Timeout:         ptypes.DurationProto(target.BuildTimeout),
		}
		actionDigest, actionMsg := digestMessageContents(action)
		ch <- &blob{Data: actionMsg, Digest: *actionDigest}
		digest = actionDigest
		return nil
	})
	return
}

// buildCommand builds the command for a single target.
func (c *Client) buildCommand(target *core.BuildTarget, stamp []byte) *pb.Command {
	return &pb.Command{
		Platform: &pb.Platform{
			Properties: []*pb.Platform_Property{
				{
					Name:  "OSFamily",
					Value: translateOS(target.Subrepo),
				},
				// We don't really keep information around about ISA. Can look at adding
				// that later if it becomes relevant & interesting.
			},
		},
		// We have to run everything through bash since our commands are arbitrary.
		// Unfortunately we can't just say "bash", we need an absolute path which is
		// a bit weird since it assumes that our absolute path is the same as the
		// remote one (which is probably OK on the same OS, but not between say Linux and
		// FreeBSD where bash is not idiomatically in the same place).
		Arguments: []string{
			c.bashPath, "--noprofile", "--norc", "-u", "-o", "pipefail", "-c", target.GetCommand(c.state),
		},
		EnvironmentVariables: buildEnv(c.state, target, stamp),
		OutputFiles:          target.Outputs(),
		// TODO(peterebden): We will need to deal with OutputDirectories somehow.
		//                   Unfortunately it's unclear how to do that without introducing
		//                   a requirement on our rules that they specify them explicitly :(
	}
}

// buildInputRoot constructs the directory that is the input root and optionally uploads it.
func (c *Client) buildInputRoot(target *core.BuildTarget, upload bool) (*pb.Directory, error) {
	return &pb.Directory{}, nil
}

// translateOS converts the OS name of a subrepo into a Bazel-style OS name.
func translateOS(subrepo *core.Subrepo) string {
	if subrepo == nil {
		return reallyTranslateOS(runtime.GOOS)
	}
	return reallyTranslateOS(subrepo.Arch.OS)
}

func reallyTranslateOS(os string) string {
	switch os {
	case "darwin":
		return "macos"
	default:
		return os
	}
}

// buildEnv creates the set of environment variables for this target.
func buildEnv(state *core.BuildState, target *core.BuildTarget, stamp []byte) []*pb.Command_EnvironmentVariable {
	env := core.StampedBuildEnvironment(state, target, stamp)
	sort.Strings(env) // Proto says it must be sorted (not just consistently ordered :( )
	vars := make([]*pb.Command_EnvironmentVariable, len(env))
	for i, e := range env {
		idx := strings.IndexByte(e, '=')
		vars[i] = &pb.Command_EnvironmentVariable{
			Name:  e[:idx],
			Value: e[idx+1:],
		}
	}
	return vars
}
