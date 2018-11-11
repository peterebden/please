// +build bootstrap

package remote

import "fmt"

func Build(tid int, state *core.BuildState, target *core.BuildTarget, hash []byte) error {
	return fmt.Errorf("Remote workers not compiled")
}
