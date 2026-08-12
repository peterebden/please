package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/thought-machine/please/src/core"
)

// cycleCheckDuration is the length of time we allow inactivity for before we trigger cycle detection.
const cycleCheckDuration = 5 * time.Second

type cycleDetector struct {
	graph   *core.BuildGraph
	stopped bool
}

// Check runs a single check of the build graph to see if any cycles can be detected.
// If it finds one an errCycle is returned.
func (c *cycleDetector) Check() *errCycle {
	if c.stopped {
		return nil
	}
	log.Debug("Running cycle detection...")
	complete := map[*core.BuildTarget]struct{}{}
	partial := map[*core.BuildTarget]struct{}{}

	// visit visits a target and all its transitive dependencies. As each is visited they are marked as
	// partially visited; when we bottom out a tree successfully we mark it as completely visited (this
	// saves us from revisiting any node we've successfully visited before).
	// If a cycle is found it returns a slice of the targets in that cycle, and a bool indicating if the
	// cycle is complete or not (if not the caller will need to add its node to it as well).
	var visit func(target *core.BuildTarget) ([]*core.BuildTarget, bool)
	visit = func(target *core.BuildTarget) ([]*core.BuildTarget, bool) {
		if c.stopped {
			return nil, false
		} else if _, present := complete[target]; present {
			return nil, false
		} else if _, present := partial[target]; present {
			return []*core.BuildTarget{target}, false
		}
		partial[target] = struct{}{}
		// Ignore anything we can't resolve; we run while the build is still going on so it's
		// entirely normal for parts of the graph not to exist yet.
		deps, _ := target.Dependencies(c.graph)
		for _, dep := range deps {
			if cycle, done := visit(dep); cycle != nil {
				if done || target == cycle[len(cycle)-1] {
					return cycle, true // This target is already in the cycle
				}
				return append([]*core.BuildTarget{target}, cycle...), false
			}
		}
		delete(partial, target)
		complete[target] = struct{}{}
		return nil, false
	}

	for _, target := range c.graph.AllTargets() {
		if c.stopped {
			log.Debug("Cycle detection terminated")
			return nil
		}
		if _, present := complete[target]; !present {
			if cycle, _ := visit(target); cycle != nil {
				log.Debug("Cycle detection complete, cycle found: %s", cycle)
				return &errCycle{Cycle: cycle}
			}
		}
	}
	log.Debug("Cycle detection complete, no cycles found")
	return nil
}

// Stop stops any existing run of the cycle detector.
func (c *cycleDetector) Stop() {
	c.stopped = true
}

// An errCycle is emitted when a graph cycle is detected.
type errCycle struct {
	Cycle []*core.BuildTarget
}

func (err *errCycle) Error() string {
	labels := make([]string, len(err.Cycle)+1)
	for i, t := range err.Cycle {
		labels[i] = t.Label.String()
	}
	labels[len(labels)-1] = labels[0]
	return fmt.Sprintf("Dependency cycle found:\n%s\nSorry, but you'll have to refactor your build files to avoid this cycle", strings.Join(labels, "\n -> "))
}

// checkForCycles consumes a stream of build results and triggers cycle detection when appropriate
func checkForCycles(state *core.BuildState, results <-chan *core.BuildResult, cancel context.CancelCauseFunc) {
	activeTargets := map[*core.BuildTarget]struct{}{}
	t := time.NewTimer(cycleCheckDuration)
	t.Stop()
	var result *BuildResult
	for {
		if len(activeTargets) == 0 {
			t.Reset(cycleCheckDuration)
			select {
			case result = <-results:
				// This has to be properly managed to prevent hangs.
				if !t.Stop() {
					<-t.C
				}
			case <-t.C:
				go state.checkForCycles()
				go dumpGoroutineInfo()
				// Still need to get a result!
				result = <-state.progress.internalResults
			}
		} else {
			result = <-state.progress.internalResults
		}
		if target := result.target; target != nil {
			if result.Status.IsActive() {
				activeTargets[target] = struct{}{}
			} else {
				delete(activeTargets, target)
			}
		}
	}
}
