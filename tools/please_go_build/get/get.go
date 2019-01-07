// Package get implements a simple replacement for the compiling part of 'go get'.
// We use this to install the code after downloading it.
//
// Since this is used for fetching third-party code, it cannot itself have any
// third-party dependencies.
package get

import (
	"os"
	"path/filepath"
)

// Get implements the compiling part of 'go get'.
// Essentially this just walks the given tmpdir, finds any directories with Go files in
// them and compiles them.
func Get(tmpdir string, binary bool) error {
	return filepath.Walk(tmpdir, func(path string, info os.FileInfo, err error) error
}
