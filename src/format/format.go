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
	if cli.Contains(basename, state.Config.Parse.BuildFileName) || strings.HasSuffix(basename, ".build_defs") {
		return ReformatBuild(filename)
	}
	for name, format := range state.Config.Format {
		for _, pattern := range format.Pattern {
			if matches, err := filepath.Glob(pattern, basename); err != nil {
				log.Warning("Bad glob pattern for %s: %s", name, err)
			} else if len(matches) > 0 {
				log.Notice("Reformatting %s as %s...", filename, name)
				return reformat(filename, format.Command, format.InPlace)
			}
		}
	}
	return fmt.Errorf("No format pattern matched for %s", filename)
}

// reformat reformats a single file given a command.
func format(filename, command string, inPlace bool) error {
	tokens, err := shlex.Split(command)
	if err != nil {
		return err
	}
	cmd := exec.Command(tokens[0], tokens[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to reformat %s: %s", filename, cmd.Run())
	} else if inPlace {
		return nil
	} else if err := fs.WriteFile(bytes.NewReader(out), filename, 0644); err != nil {
		return fmt.Errorf("Failed to write reformatted file %s: %s", filename, err)
	}
	return nil
}

// ReformatBuild reformats a build file in-place.
func ReformatBuild(filename string) error {
	ast, err := asp.NewParser(nil).ParseFileOnly(filename)
	if err != nil {
		return err
	}
	return fmt.Errorf("not implemented %s", ast)
}
