// Package lint implements the `plz lint` command which invokes linters on build targets.
package lint

import (
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/peterebden/go-deferred-regex"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/process"
)

var log = logging.MustGetLogger("lint")

// Lint implements the core logic for linting a single target.
func Lint(tid int, state *core.BuildState, label core.BuildLabel, remote bool, linter string) {
	target := state.Graph.TargetOrDie(label)
	// This is a little weird; the first time we get called with no linter set. This package
	// then decides what linters it is going to run for the relevant target and submits zero
	// or more additional tasks for them.
	if linter == "" {
		dispatchLintTasks(state, target)
		return
	}
	defer state.TaskDone()
	if err := lint(tid, state, target, remote, linter); err != nil {
		state.LogBuildError(tid, label, core.TargetLintFailed, err, "Lint failed: %s", err)
	}
}

// dispatchLintTasks determines what linters to run for this target and dispatches tasks for them.
func dispatchLintTasks(state *core.BuildState, target *core.BuildTarget) {
	// The deferred regexes can panic if the expressions are invalid, so handle that _slightly_
	// more gracefully here.
	defer func() {
		if r := recover(); r != nil {
			log.Fatalf("%s", r)
		}
	}()

	var wg sync.WaitGroup
	for name, linter := range state.Config.Linter {
		if shouldInclude(linter, target) {
			wg.Add(1)
			go func(name string, linter *core.Linter, target *core.BuildTarget) {
				defer wg.Done()
				if !linter.Target.IsEmpty() {
					// Need to wait for this thing to be ready.
					state.WaitForBuiltTarget(linter.Target, target.Label)
				}
				state.AddPendingLint(target, name)
			}(name, linter, target)
		}
	}
	// Don't actually wait for this to complete (we need to release the worker to do other things)
	// but don't mark the task as done until all the goroutines have cleaned up. This ensures the
	// build doesn't terminate while they're still pending.
	go func() {
		wg.Wait()
		state.TaskDone()
	}()
}

// shouldInclude returns true if this linter should include any inputs of this target.
func shouldInclude(linter *core.Linter, target *core.BuildTarget) bool {
	// This would be slightly nicer if we had a more unified way of iterating these things.
	for _, src := range target.Sources {
		if matchOne(linter.Include, src) && !matchOne(linter.Exclude, src) {
			return true
		}
	}
	for _, srcs := range target.NamedSources {
		for _, src := range srcs {
			if matchOne(linter.Include, src) && !matchOne(linter.Exclude, src) {
				return true
			}
		}
	}
	return false
}

// matchOne returns true if any one of these regexes matches the given path.
func matchOne(regexes []deferredregex.DeferredRegex, src core.BuildInput) bool {
	// The explicit cast is a bit dodgy but we only want to deal with inputs within the source tree.
	if fl, ok := src.(core.FileLabel); ok {
		filename := path.Join(fl.Package, fl.File)
		for _, re := range regexes {
			if re.MatchString(filename) {
				return true
			}
		}
	}
	return false
}

// lint performs the logic of linting a single target.
func lint(tid int, state *core.BuildState, target *core.BuildTarget, remote bool, linterName string) error {
	// TODO(peterebden): Remote execution support.
	linter := state.Config.Linter[linterName]
	state.LogBuildResult(tid, target, core.TargetLinting, fmt.Sprintf("Preparing to run %s...", linterName))

	tmpDir := path.Join(core.RepoRoot, target.LintDir(linterName))
	if err := prepareDirectory(state, tmpDir); err != nil {
		return err
	}
	if err := prepareSources(state, state.Graph, target, tmpDir); err != nil {
		return err
	}
	if !linter.Target.IsEmpty() {
		if err := prepareOutputs(state, state.Graph.TargetOrDie(linter.Target), tmpDir); err != nil {
			return err
		}
	}

	state.LogBuildResult(tid, target, core.TargetLinting, fmt.Sprintf("Running %s...", linterName))
	if err := runLint(state, target, tmpDir, linterName, linter); err != nil {
		return err
	}
	state.LogBuildResult(tid, target, core.TargetLinted, fmt.Sprintf("Finished %s", linterName))
	return nil
}

// runLint runs the linter over a prepared temp dir.
func runLint(state *core.BuildState, target *core.BuildTarget, tmpDir, linterName string, linter *core.Linter) error {
	srcs := target.AllSourcePaths(state.Graph)
	if !linter.Reformat {
		return runLintOnce(state, target, tmpDir, linterName, linter, srcs)
	}
	// Reformatting linters want to see each file individually.
	for _, src := range srcs {
		if err := runLintOnce(state, target, tmpDir, linterName, linter, []string{src}); err != nil {
			return err
		}
	}
	return nil
}

// runLintOnce runs a linter command once on a prepared temp dir.
func runLintOnce(state *core.BuildState, target *core.BuildTarget, tmpDir, linterName string, linter *core.Linter, srcs []string) error {
	cmd, err := command(state.Graph, linter)
	if err != nil {
		return err
	}
	env := core.LintEnvironment(state, target, tmpDir, srcs)
	log.Debug("Linting target %s\nENVIRONMENT:\n%s\n%s", target, env, cmd)
	out, _, err := state.ProcessExecutor.ExecWithTimeoutShell(target, tmpDir, env, target.BuildTimeout, state.ShowAllOutput, false, process.NewSandboxConfig(target.Sandbox, target.Sandbox), cmd)
	if err == nil && len(out) == 0 {
		return nil // assume everything is successful
	}
	if !linter.Reformat {
		target.AddLintResults(parseLintLines(linter, linterName, string(out)))
		state.LintFailed = true
		return nil
	} else if state.WriteLinterSuggestions {
		// Rewrite the linter output into the file. Note that this doesn't set state.LintFailed
		// having failed so the exit code is 0 if all the linters rewrote their suggestions.
		return os.WriteFile(srcs[0], out, fileMode(srcs[0]))
	}

	existing, err := os.ReadFile(srcs[0])
	if err != nil {
		return err
	}
	target.AddLintResults(computeDiffs(linterName, srcs[0], string(existing), string(out)))
	state.LintFailed = true
	return nil
}

// fileMode returns the mode we should write a file in.
func fileMode(filename string) os.FileMode {
	if info, err := os.Stat(filename); err == nil {
		return info.Mode()
	}
	return 0644 // Just assume this, we'll probably fail to write it in a sec if we couldn't stat() it.
}
