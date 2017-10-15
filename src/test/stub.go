package test

import "core"
import "fmt"

// runTestRemotely is a stub used during initial bootstrap when protobufs aren't available.
func runTestRemotely(state *core.BuildState, target *core.BuildTarget) ([]byte, [][]byte, []byte, error) {
	return nil, nil, nil, fmt.Errorf("Cannot run test remotely, remote running is not compiled")
}
