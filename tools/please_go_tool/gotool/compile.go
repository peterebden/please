// Package gotool implements a helper for Go compilation that mimics parts of go build.
package gotool

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/op/go-logging.v1"
)

var log = logging.MustGetLogger("gotool")

// envRegex is used in ReplaceEnv below.
// Note that this is a little quick and dirty, we should really have two disjoint
// replacements (one for ${} and one for just $) but we assume it's not necessary.
var envRegex = regexp.MustCompile(`\$\{[^/}]+`)

// LinkPackages finds any Go packages in the given temp dir and links them up a directory when needed.
// This is required for Go's import machinery to work, which can't be overridden in go tool compile
// in any useful way. The issue is effectively that we do everything within directories but Go outputs
// them a level up; i.e. src/core/BUILD in this repo outputs src/core/core.a, but Go wants it to be
// src/core.a and there's no way of altering that behaviour.
// Hence symlinking some files seems like the best option...
//
// Note that this is not *always* the case though; we also often have proto rules outputting multiple
// libraries in a directory which are imported with a more specific path. Please strives not to force
// users to choose any specific mechanism, but it is difficult to reconcile that with Go's strict path
// resolution rules.
func LinkPackages(tmpDir string) error {
	return filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if !info.IsDir() && strings.HasSuffix(path, ".a") {
			// Package archive, link it up.
			dir, file := filepath.Split(path)
			dest := filepath.Join(filepath.Dir(strings.TrimRight(dir, "/")), file)
			err := os.Symlink(path, dest)
			if err != nil && os.IsExist(err) {
				// This happens sometimes, we can't guarantee that all libraries are unique
				// at their parent. Ignore it here, the user may get import errors later but
				// there's not much we can do about that.
				log.Notice("Ignoring already-existing library %s: %s", dest, err)
				return nil
			}
			return err
		}
		return nil
	})
}

// AnnotateCoverage annotates a set of source files with coverage instrumentation.
// This is done via go tool cover, this function simply automates calling it.
func AnnotateCoverage(tool string, srcs []string) error {
	// Do them all in parallel, why not.
	var wg sync.WaitGroup
	wg.Add(len(srcs))
	var ret error
	for _, src := range srcs {
		go func(src string) {
			defer wg.Done() // However we get out of here, this file is done with.
			v := strings.Replace(filepath.Base(src), ".", "_", -1)
			cmd := exec.Command(tool, "tool", "cover", "-mode", "set", "-var", v, src)
			out, err := cmd.Output()
			if err != nil {
				ret = fmt.Errorf("Failed to annotate coverage for %s: %s", src, err)
				return
			}
			// Remove the existing file. It might be a linked source file, we cannot change it.
			if err := os.Remove(src); err != nil {
				ret = fmt.Errorf("Failed to remove source file: %s", err)
				return
			}
			// Write the modified version back to the file
			if err := ioutil.WriteFile(src, out, 0644); err != nil {
				ret = fmt.Errorf("Failed to write updated file: %s", err)
			}
		}(src)
	}
	wg.Wait()
	return ret
}

// ReplaceEnv replaces shell-style variables in a string with environment variables.
// We use this to support things like $TMP_DIR in GOPATH.
func ReplaceEnv(s string) string {
	return envRegex.ReplaceAllStringFunc(s, func(s string) string {
		return os.Getenv(strings.Trim(s, "${}"))
	})
}
