// Package main implements a simple cut-down version of the please_go_build tool which
// is needed to build the third_party dependencies for everything else.
// It has to not depend on any of them itself which is obviously rather awkward.
package main

import (
	"fmt"
	"os"

	"github.com/thought-machine/please/tools/please_go_build/get"
)

func main() {
	if err := get.Get(os.Getenv("TMP_DIR"), false); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build: %s\n", err)
		os.Exit(1)
	}
}
