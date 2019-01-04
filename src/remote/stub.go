// +build bootstrap

package remote

import "fmt"

// Build is a stub used at bootstrapping time.
func Build(tid int, state *core.BuildState, target *core.BuildTarget, hash []byte) error {
	return fmt.Errorf("Remote workers not compiled")
}
