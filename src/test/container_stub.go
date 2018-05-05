//+build bootstrap

package test

import "core"
import "fmt"

func runContainerisedTest(state *core.BuildState, target *core.BuildTarget) ([]byte, error) {
	return nil, fmt.Errorf("Containerisation not supported during bootstrap")
}
