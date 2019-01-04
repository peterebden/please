// Tests for general parse functions.

package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thought-machine/please/src/core"
)

var empty = []string{}

func TestAddDepSimple(t *testing.T) {
	// Simple case with only one package parsed and one target added
	state := makeState(true, false)
	activateTarget(state, nil, buildLabel("//package1:target1"), core.OriginalTarget, false, false, empty, empty)
	assertPending(t, state, []string{"//package2:target1", "//package2:target1"}, nil)
	assert.Equal(t, 5, state.NumActive())
}

func TestAddDepMultiple(t *testing.T) {
	// Similar to above but doing all targets in that package
	state := makeState(true, false)
	activateTarget(state, nil, buildLabel("//package1:target1"), core.OriginalTarget, false, false, empty, empty)
	activateTarget(state, nil, buildLabel("//package1:target2"), core.OriginalTarget, false, false, empty, empty)
	activateTarget(state, nil, buildLabel("//package1:target3"), core.OriginalTarget, false, false, empty, empty)
	// We get an additional dep on target2, but not another on package2:target1 because target2
	// is already activated since package1:target1 depends on it
	assertPending(t, state, []string{"//package2:target1", "//package2:target1", "//package2:target2"}, nil)
	assert.Equal(t, 7, state.NumActive())
}

func TestAddDepMultiplePackages(t *testing.T) {
	// This time we already have package2 parsed
	state := makeState(true, true)
	activateTarget(state, nil, buildLabel("//package1:target1"), core.OriginalTarget, false, false, empty, empty)
	assertPending(t, state, nil, []string{"//package2:target2"})
	assert.Equal(t, 6, state.NumActive())
}

func TestAddDepNoBuild(t *testing.T) {
	// Tag state as not needing build. We shouldn't get any pending builds at this point.
	state := makeState(true, true)
	state.NeedBuild = false
	activateTarget(state, nil, buildLabel("//package1:target1"), core.OriginalTarget, false, false, empty, empty)
	assertPending(t, state, nil, nil)
	assert.Equal(t, 1, state.NumActive()) // Parses only
}

func TestAddParseDep(t *testing.T) {
	// Tag state as not needing build. Any target that needs to be built to complete parse
	// should still get queued for build though. Recall that we indicate this with :all...
	state := makeState(true, true)
	state.NeedBuild = false
	activateTarget(state, nil, buildLabel("//package2:target2"), buildLabel("//package3:all"), false, false, empty, empty)
	assertPending(t, state, nil, []string{"//package2:target2"})
	assert.Equal(t, 2, state.NumActive())
}

func TestAddDepRescan(t *testing.T) {
	// Simulate a post-build function and rescan.
	state := makeState(true, true)
	parses, builds, _ := state.TaskQueues()
	activateTarget(state, nil, buildLabel("//package1:target1"), core.OriginalTarget, false, false, empty, empty)
	assert.Equal(t, core.ParseBuildLabel("//package2:target2", ""), <-builds)
	assert.Equal(t, 6, state.NumActive())

	// Add new target & dep to target1
	target4 := makeTarget("//package1:target4")
	state.Graph.Package("package1", "").AddTarget(target4)
	state.Graph.AddTarget(target4)
	target1 := state.Graph.TargetOrDie(buildLabel("//package1:target1"))
	target1.AddDependency(buildLabel("//package1:target4"))
	state.Graph.AddDependency(buildLabel("//package1:target1"), buildLabel("//package1:target4"))

	// Fake test: calling this now should have no effect because rescan is not true.
	addDep(state, buildLabel("//package1:target1"), core.OriginalTarget, false, false)
	assertNothingPending(t, state, parses, builds)

	// Now running this should activate it
	rescanDeps(state, map[*core.BuildTarget]struct{}{target1: {}})
	assertPendingQueues(t, state, parses, builds, nil, []string{"//package1:target4"})
	assert.True(t, state.Graph.AllDependenciesResolved(target1))
}

func TestAddParseDepDeferred(t *testing.T) {
	// Similar to TestAddParseDep but where we scan the package once and come back later because
	// something else asserts a dependency on it.
	state := makeState(true, true)
	parses, builds, _ := state.TaskQueues()
	state.NeedBuild = false
	assert.Equal(t, 1, state.NumActive())
	activateTarget(state, nil, buildLabel("//package2:target2"), core.OriginalTarget, false, false, empty, empty)
	assertNothingPending(t, state, parses, builds)

	// Now the undefer kicks off...
	activateTarget(state, nil, buildLabel("//package2:target2"), buildLabel("//package1:all"), false, false, empty, empty)
	assertPendingQueues(t, state, parses, builds, nil, []string{"//package2:target2"}) // This time!
	assert.Equal(t, 2, state.NumActive())
}

func makeTarget(label string, deps ...string) *core.BuildTarget {
	target := core.NewBuildTarget(core.ParseBuildLabel(label, ""))
	for _, dep := range deps {
		target.AddDependency(core.ParseBuildLabel(dep, ""))
	}
	return target
}

// makeState creates a new build state with optionally one or two packages in it.
// Used in various tests above.
func makeState(withPackage1, withPackage2 bool) *core.BuildState {
	state := core.NewBuildState(5, nil, 4, core.DefaultConfiguration())
	if withPackage1 {
		pkg := core.NewPackage("package1")
		state.Graph.AddPackage(pkg)
		pkg.AddTarget(makeTarget("//package1:target1", "//package1:target2", "//package2:target1"))
		pkg.AddTarget(makeTarget("//package1:target2", "//package2:target1"))
		pkg.AddTarget(makeTarget("//package1:target3", "//package2:target2"))
		state.Graph.AddTarget(pkg.Target("target1"))
		state.Graph.AddTarget(pkg.Target("target2"))
		state.Graph.AddTarget(pkg.Target("target3"))
		addDeps(state.Graph, pkg)
	}
	if withPackage2 {
		pkg := core.NewPackage("package2")
		state.Graph.AddPackage(pkg)
		pkg.AddTarget(makeTarget("//package2:target1", "//package2:target2", "//package1:target3"))
		pkg.AddTarget(makeTarget("//package2:target2"))
		state.Graph.AddTarget(pkg.Target("target1"))
		state.Graph.AddTarget(pkg.Target("target2"))
		addDeps(state.Graph, pkg)
	}
	return state
}

func addDeps(graph *core.BuildGraph, pkg *core.Package) {
	for _, target := range pkg.AllTargets() {
		for _, dep := range target.DeclaredDependencies() {
			graph.AddDependency(target.Label, dep)
		}
	}
}

func assertPending(t *testing.T, state *core.BuildState, parseTargets, buildTargets []string) {
	parses, builds, _ := state.TaskQueues()
	assertPendingQueues(t, state, parses, builds, parseTargets, buildTargets)
}

func assertPendingQueues(t *testing.T, state *core.BuildState, parses <-chan core.LabelPair, builds <-chan core.BuildLabel, parseTargets, buildTargets []string) {
	state.StopAll()
	pendingParses := []core.BuildLabel{}
	pendingBuilds := []core.BuildLabel{}
	for task := range parses {
		pendingParses = append(pendingParses, task.Label)
	}
	for label := range builds {
		pendingBuilds = append(pendingBuilds, label)
	}
	assert.Equal(t, toLabels(parseTargets), pendingParses)
	assert.Equal(t, toLabels(buildTargets), pendingBuilds)
}

func assertNothingPending(t *testing.T, state *core.BuildState, parses <-chan core.LabelPair, builds <-chan core.BuildLabel) {
	pendingParses := []core.BuildLabel{}
	pendingBuilds := []core.BuildLabel{}
loop:
	for {
		select {
		case task := <-parses:
			pendingParses = append(pendingParses, task.Label)
		case label := <-builds:
			pendingBuilds = append(pendingBuilds, label)
		default:
			break loop
		}
	}
	assert.Equal(t, []core.BuildLabel{}, pendingParses)
	assert.Equal(t, []core.BuildLabel{}, pendingBuilds)
}

func buildLabel(bl string) core.BuildLabel {
	return core.ParseBuildLabel(bl, "")
}

func toLabels(s []string) []core.BuildLabel {
	ret := make([]core.BuildLabel, len(s))
	for i, x := range s {
		ret[i] = core.ParseBuildLabel(x, "")
	}
	return ret
}
