// Package get implements a simple replacement for the compiling part of 'go get'.
// We use this to install the code after downloading it.
//
// Since this is used for fetching third-party code, it cannot itself have any
// third-party dependencies.
package get

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Get implements the compiling part of 'go get'.
// Essentially this just walks the given tmpdir, finds any directories with Go files in
// them and compiles them.
func Get(tmpdir string, binary bool) error {
	pkgs := []*pkg{}
	if err := filepath.Walk(tmpdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if info.IsDir() {
			if pkg, error := readPackage(path); err != nil {
				return err
			} else if len(pkg.Srcs) > 0 {
				pkgs = append(pkgs, pkg)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return fmt.Errorf("%s", pkgs)
}

// readPackage reads a directory containing a single Go package and returns its definition.
func readPackage(path string) (*pkg, error) {
	p := &pkg{Path: path}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
}

type pkg struct {
	Name, Path string
	Srcs       []string
	Deps       []*pkg
}
