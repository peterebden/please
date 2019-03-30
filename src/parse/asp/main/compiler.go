// Package main implements a compiler for the builtin build rules, which is used at bootstrap time.
package main

import (
	"io/ioutil"
	"os"
	"path"

	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/cli"
	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/parse/asp"
	"github.com/thought-machine/please/src/parse/asp/compiler"
)

var log = logging.MustGetLogger("asp")

var opts = struct {
	Usage     string
	Verbosity cli.Verbosity `short:"v" long:"verbosity" default:"notice" description:"Verbosity of output (higher number = more output)"`
	OutputDir string        `short:"o" long:"output_dir" default:"." description:"Output directory"`
	Go        bool          `short:"g" long:"go" description:"Compile to Go code"`
	Args      struct {
		BuildFiles []string `positional-arg-name:"files" required:"true" description:"BUILD files to parse"`
	} `positional-args:"true"`
}{
	Usage: `Compiler for built-in build rules.`,
}

func check(err error) {
	if err != nil {
		log.Fatalf("%s", err)
	}
}

func main() {
	cli.ParseFlagsOrDie("parser", &opts)
	cli.InitLogging(opts.Verbosity)

	check(os.MkdirAll(opts.OutputDir, os.ModeDir|0775))
	p := asp.NewParser(core.NewDefaultBuildState())
	if opts.Go {
		stmts := []*asp.Statement{}
		for _, filename := range opts.Args.BuildFiles {
			parsed, err := p.ParseFileOnly(filename)
			check(err)
			stmts = append(stmts, parsed...)
		}
		b, err := compiler.Compile(stmts)
		check(err)
		check(ioutil.WriteFile(path.Join(opts.OutputDir, "rules.go"), b, 0644))
		check(ioutil.WriteFile(path.Join(opts.OutputDir, "types.go"), []byte(compiler.TypesFile), 0644))
	} else {
		for _, filename := range opts.Args.BuildFiles {
			out := path.Join(opts.OutputDir, path.Base(filename)) + ".gob"
			check(p.ParseToFile(filename, out))
		}
	}
}
