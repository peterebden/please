package query

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thought-machine/please/src/core"
)

func TestReverseDeps(t *testing.T) {
	state := core.NewDefaultBuildState()
	graph := state.Graph

	root := core.NewBuildTarget(core.ParseBuildLabel("//package:root", ""))
	branch := core.NewBuildTarget(core.ParseBuildLabel("//package:branch", ""))
	leaf := core.NewBuildTarget(core.ParseBuildLabel("//package:leaf", ""))
	branch.AddDependency(root.Label)
	leaf.AddDependency(branch.Label)
	graph.AddTarget(root)
	graph.AddTarget(branch)
	graph.AddTarget(leaf)
	branch.ResolveDependencies(graph)
	leaf.ResolveDependencies(graph)

	pkg := core.NewPackage("package")
	pkg.AddTarget(root)
	pkg.AddTarget(branch)
	pkg.AddTarget(leaf)
	graph.AddPackage(pkg)

	labels := revDepsLabels(state, []core.BuildLabel{branch.Label}, -1)
	assert.ElementsMatch(t, core.BuildLabels{leaf.Label}, labels)
	labels = revDepsLabels(state, []core.BuildLabel{root.Label}, -1)
	assert.ElementsMatch(t, core.BuildLabels{branch.Label, leaf.Label}, labels)
	labels = revDepsLabels(state, []core.BuildLabel{root.Label}, 1)
	assert.ElementsMatch(t, core.BuildLabels{branch.Label}, labels)
}

func revDepsLabels(state *core.BuildState, labels []core.BuildLabel, levelsToGo int) core.BuildLabels {
	ls := map[core.BuildLabel]int{}
	getRevDepTransitiveLabels(state, labels, ls, levelsToGo)

	ret := make([]core.BuildLabel, 0, len(ls))
	for l := range ls {
		ret = append(ret, l)
	}
	return ret
}
