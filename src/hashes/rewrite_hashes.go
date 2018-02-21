// +build nobootstrap

package hashes

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"runtime"

	"gopkg.in/op/go-logging.v1"

	"build"
	"core"
	"parse/asp"
)

var log = logging.MustGetLogger("hashes")

// RewriteHashes rewrites the hashes in a BUILD file.
func RewriteHashes(state *core.BuildState, labels []core.BuildLabel) {
	// Collect the targets per-package so we only rewrite each file once.
	m := map[string]map[string]string{}
	for _, l := range labels {
		for _, target := range state.Graph.PackageOrDie(l.PackageName).AllChildren(state.Graph.TargetOrDie(l)) {
			// Ignore targets with no hash specified.
			if len(target.Hashes) == 0 {
				continue
			}
			h, err := build.OutputHash(target)
			if err != nil {
				log.Fatalf("%s\n", err)
			}
			// Interior targets won't appear in the BUILD file directly, look for their parent instead.
			l := target.Label.Parent()
			hashStr := hex.EncodeToString(h)
			if m2, present := m[l.PackageName]; present {
				m2[l.Name] = hashStr
			} else {
				m[l.PackageName] = map[string]string{l.Name: hashStr}
			}
		}
	}
	for pkgName, hashes := range m {
		if err := rewriteHashes(state, state.Graph.PackageOrDie(pkgName).Filename, runtime.GOOS+"_"+runtime.GOARCH, hashes); err != nil {
			log.Fatalf("%s\n", err)
		}
	}
}

// rewriteHashes rewrites hashes in a single file.
func rewriteHashes(state *core.BuildState, filename, platform string, hashes map[string]string) error {
	log.Notice("Rewriting hashes in %s...", filename)
	p := asp.NewParser(nil)
	stmts, err := p.ParseFileOnly(filename)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	lines := bytes.Split(b, []byte{'\n'})
	for k, v := range hashes {
		if err := rewriteHash(lines, stmts, platform, k, v); err != nil {
			return err
		}
	}
	return nil
}

// rewriteHash rewrites a single hash on a statement.
func rewriteHash(lines [][]byte, stmts []*asp.Statement, platform, name, hash string) error {
	stmt := asp.FindTarget(stmts, name)
	if stmt == nil {
		return fmt.Errorf("Can't find target %s to rewrite", name)
	} else if arg := asp.FindArgument(stmt, "hashes"); arg != nil && arg.Value.Val != nil && arg.Value.Val.List != nil {
		for _, h := range arg.Value.Val.List.Values {
			if h.Val != nil && h.Val.String != "" {
				lines[h.Pos.Line-1] = rewriteLine(lines[h.Pos.Line-1], h.Pos.Column, h.Val.String, hash)
				return nil
			}
		}
	} else if arg := asp.FindArgument(stmt, "hash"); arg != nil && arg.Value.Val != nil && arg.Value.Val.String != "" {
		h := arg.Value
		lines[h.Pos.Line-1] = rewriteLine(lines[h.Pos.Line-1], h.Pos.Column, h.Val.String, hash)
		return nil
	}
	return fmt.Errorf("Can't find hash or hashes argument on %s", name)
}

// rewriteLine implements the rewriting logic within a single line.
func rewriteLine(line []byte, start int, current, new string) []byte {
	start -= 1 // columns are 1-indexed
	return append(append(line[:start], []byte(new)...), line[start+len(current):]...)
}
