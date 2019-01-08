// Package get implements a replacement for part of "go get" that finds code packages
// and emits instructions about how to build it. It does not invoke compilation itself
// in order not to have to wrangle dependency order and delegate lots of real work to
// the existing plz rules (for example, we don't want to reinvent the build steps needed
// for cgo / asm).
//
// Since this is used for fetching third-party code, it cannot itself have any
// third-party dependencies.
package get

import (
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Get implements the finding part of 'go get'.
// Essentially this just walks the given tmpdir, finds any directories with Go files in
// them and emits instructions about what to compile.
func Get(tmpdir, basePkg string, binary bool) error {
	j := func(s []string) string { return strings.TrimSpace(strings.Join(s, ",")) }
	return filepath.Walk(tmpdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if info.IsDir() {
			pkgName, _ := filepath.Rel(tmpdir, path)
			if p, err := readPackage(path, pkgName, basePkg); err != nil {
				return err
			} else if len(p.CgoSrcs) > 0 {
				fmt.Printf("%s|%s|%s|%s|%s|%s|%s\n", p.Name, p.Path, j(p.CgoSrcs), j(p.Srcs), j(p.CSrcs), j(p.Hdrs), j(p.Deps))
			} else if len(p.Srcs) > 0 {
				fmt.Printf("%s|%s|%s|%s|%s\n", p.Name, p.Path, j(p.Srcs), j(p.AsmSrcs), j(p.Deps))
			}
		}
		return nil
	})
}

// readPackage reads a directory containing a single Go package and returns its definition.
func readPackage(path, pkgName, basePkg string) (*pkg, error) {
	p := &pkg{
		Path: path,
		Name: strings.Replace(strings.Replace(pkgName, "/", "_", -1), ".", "_", -1),
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		name := f.Name()
		if ext := filepath.Ext(name); ext == ".go" {
			// Go files can be filtered out.
			if ok, err := build.Default.MatchFile(path, name); ok && err == nil {
				if strings.HasSuffix(name, "_test.go") {
					continue // MatchFile doesn't seem to identify this.
				}
				cgo, imports := readImports(filepath.Join(path, name), basePkg)
				if cgo {
					p.CgoSrcs = append(p.CgoSrcs, name)
				} else {
					p.Srcs = append(p.Srcs, name)
				}
				p.Deps = imports
			}
		} else if ext == ".s" {
			p.AsmSrcs = append(p.AsmSrcs, name)
		} else if ext == ".c" {
			p.CSrcs = append(p.CSrcs, name)
		} else if ext == ".h" {
			p.Hdrs = append(p.Hdrs, name)
		}
	}
	return p, nil
}

// readImports reads a set of imports from a Go file.
func readImports(filename, basePkg string) (bool, []string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ImportsOnly)
	if err != nil {
		panic("Failed to parse file: " + err.Error())
	}
	cgo := false
	imports := []string{}
	for _, imp := range f.Imports {
		if imp.Path.Value == "C" {
			cgo = true
		} else if strings.HasPrefix(imp.Path.Value, basePkg) {
			// We only need to care about deps within this package
			name := strings.Trim(strings.TrimPrefix(imp.Path.Value, basePkg), "/")
			name = strings.Replace(name, "/", "_", -1)
			imports = append(imports, name)
		}
	}
	return cgo, imports
}

type pkg struct {
	Name, Path string
	Srcs       []string // Go files
	CgoSrcs    []string // Go files that import C
	AsmSrcs    []string // Assembly files
	CSrcs      []string // C files
	Hdrs       []string // Header files
	Deps       []string
}
