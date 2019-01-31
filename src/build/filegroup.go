// Logic relating to building filegroups.
//
// Unlike most targets, filegroups are special in that (1) they are known to the
// system and have a custom implementation and (2) multiple filegroups can output
// the same file. This does lead to a potential race condition where we have to
// be sure to build each output file only once.
// Currently this is implemented by a single thread that builds them all; there
// are other schemes we could have but this is simple enough (and since we link
// them rather than copying there should not be a lot of I/O wait).

package build

import (
	"os"
	"path"
	"sync"

	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/fs"
	"github.com/thought-machine/please/src/remote/fsclient"
)

// Init initialises common resources for the build package.
func Init(state *core.BuildState) {
	theFilegroupBuilder = &filegroupBuilder{
		built: map[string]bool{},
	}
	if len(state.Config.Build.RemoteFSURL) > 0 {
		theRemoteClient = fsclient.New(state.Config.Build.RemoteFSURL)
	}
}

// A filegroupBuilder is a singleton that we have that builds all filegroups.
// This works around the problem where multiple filegroups can output the same
// file, which means that if built simultaneously they can fight with one another.
type filegroupBuilder struct {
	mutex sync.Mutex
	built map[string]bool
}

var theFilegroupBuilder *filegroupBuilder
var theRemoteClient fsclient.Client

// Build builds a single filegroup file.
func (builder *filegroupBuilder) Build(state *core.BuildState, target *core.BuildTarget, from, to string) error {
	builder.mutex.Lock()
	defer builder.mutex.Unlock()
	if builder.built[to] {
		return nil // File's already been built.
	}
	if fs.IsSameFile(from, to) {
		// File exists already and is the same file. Nothing to do.
		// TODO(peterebden): This should also have a recursive case for when it's a directory...
		builder.built[to] = true
		return nil
	}
	// Must actually build the file.
	if err := os.RemoveAll(to); err != nil {
		return err
	} else if err := fs.EnsureDir(to); err != nil {
		return err
	} else if err := fs.RecursiveLink(from, to, target.OutMode()); err != nil {
		return err
	}
	builder.built[to] = true
	state.PathHasher.MoveHash(from, to, true)
	return nil
}

// buildFilegroup runs the manual build steps for a filegroup rule.
// We don't force this to be done in bash to avoid errors with maximum command lengths,
// and it's actually quite fiddly to get just so there.
func buildFilegroup(state *core.BuildState, target *core.BuildTarget) error {
	if err := prepareDirectory(target.OutDir(), false); err != nil {
		return err
	}
	for _, path := range target.FilegroupPaths(state, false) {
		if err := theFilegroupBuilder.Build(state, target, path.Src, path.Tmp); err != nil {
			return err
		}
	}
	if target.HasLabel("py") && !target.IsBinary {
		// Pre-emptively create __init__.py files so the outputs can be loaded dynamically.
		// It's a bit cheeky to do non-essential language-specific logic but this enables
		// a lot of relatively normal Python workflows.
		// Errors are deliberately ignored.
		if pkg := state.Graph.PackageByLabel(target.Label); pkg == nil || !pkg.HasOutput("__init__.py") {
			// Don't create this if someone else is going to create this in the package.
			createInitPy(target.OutDir())
		}
	}
	return nil
}

// copyFilegroupHashes copies the hashes of the inputs of this filegroup to their outputs.
// This is a small optimisation to ensure we don't need to recalculate them unnecessarily.
func copyFilegroupHashes(state *core.BuildState, target *core.BuildTarget) {
	for _, path := range target.FilegroupPaths(state, false) {
		if path.Src != path.Tmp {
			state.PathHasher.MoveHash(path.Src, path.Tmp, true)
		}
	}
}

// updateHashFilegroupPaths sets the output paths on a hash_filegroup rule.
// Unlike normal filegroups, hash filegroups can't calculate these themselves very readily.
func updateHashFilegroupPaths(state *core.BuildState, target *core.BuildTarget) {
	for _, path := range target.FilegroupPaths(state, true) {
		target.AddOutput(path.Tmp)
	}
}

func createInitPy(dir string) {
	initPy := path.Join(dir, "__init__.py")
	if core.PathExists(initPy) {
		return
	}
	if f, err := os.OpenFile(initPy, os.O_RDONLY|os.O_CREATE, 0444); err == nil {
		f.Close()
	}
	dir = path.Dir(dir)
	if dir != core.GenDir && dir != "." && !core.PathExists(path.Join(dir, "__init__.py")) {
		createInitPy(dir)
	}
}
