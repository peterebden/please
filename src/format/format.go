// Package format implements a reformatter for BUILD files, along with a simple
// configurable system to invoke programs to do the same for other files.
package format

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/buildtools/build"
	"github.com/google/shlex"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/cli"
	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/fs"
	"github.com/thought-machine/please/src/parse/asp"
)

var log = logging.MustGetLogger("format")

// Reformat reformats the given file in-place using configured rules.
func Reformat(state *core.BuildState, filename string) error {
	basename := path.Base(filename)
	if cli.ContainsString(basename, state.Config.Parse.BuildFileName) || strings.HasSuffix(basename, ".build_defs") {
		return ReformatBuild(filename)
	}
	for name, format := range state.Config.Format {
		for _, pattern := range format.Pattern {
			if matched, err := filepath.Match(pattern, basename); err != nil {
				log.Warning("Bad glob pattern for %s: %s", name, err)
			} else if matched {
				log.Notice("Reformatting %s as %s...", filename, name)
				return reformat(filename, format.Command, format.InPlace)
			}
		}
	}
	return fmt.Errorf("No format pattern matched for %s", filename)
}

// reformat reformats a single file given a command.
func reformat(filename, command string, inPlace bool) error {
	tokens, err := shlex.Split(command)
	if err != nil {
		return err
	}
	cmd := exec.Command(tokens[0], append(tokens[1:], filename)...)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to run %s %s: %s", cmd.Path, strings.Join(cmd.Args, " "), err)
	} else if inPlace {
		return nil
	} else if err := fs.WriteFile(bytes.NewReader(out), filename, 0644); err != nil {
		return fmt.Errorf("Failed to write reformatted file %s: %s", filename, err)
	}
	return nil
}

// ReformatBuild reformats a build file in-place.
func ReformatBuild(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	ast, err := asp.NewParser(nil).ParseData(data, filename)
	if err != nil {
		return err
	}
	//lines := strings.Split(string(data), "\n")
	// We only really care about function calls here; right now we don't
	// reformat much else. In time we might also try to pretty-print function definitions.
	asp.WalkAST(ast, func(stmt *asp.IdentStatement) bool {
		if stmt.Action != nil && stmt.Action.Call != nil {
			log.Debug("Here")
		}
		return true
	})
	f, err := build.ParseBuild(filename, data)
	if err != nil {
		return err
	}
	return fs.WriteFile(bytes.NewReader(build.Format(f)), filename, 0644)
}

type formatted struct {
	Start, End  int // original line numbers
	Replacement []byte
}
