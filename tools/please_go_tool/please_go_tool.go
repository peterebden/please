// Package main implements a multipurpose helper for compiling Go code.
// This is mostly intended to implement a level of logic that 'go build' would normally do
// without having to implement all of that in bash.
package main

import (
	"os"
	"syscall"

	"gopkg.in/op/go-logging.v1"

	"cli"
	"tools/please_go_tool/gotool"
)

var log = logging.MustGetLogger("plz_go_tool")

var opts struct {
	Verbosity int      `short:"v" long:"verbose" default:"1" description:"Verbosity of output (higher number = more output, default 1 -> warnings and errors only)"`
	TmpDir    string   `long:"tmp_dir" env:"TMP_DIR" required:"true" description:"Temp dir that we're running in"`
	Sources   []string `long:"srcs" env:"SRCS" description:"Source files" required:"true"`

	Compile struct {
		Cover bool   `short:"c" long:"cover" description:"Annotate files for coverage"`
		Out   string `short:"o" long:"out" env:"OUT" description:"Output archive file"`
		Args  struct {
			Go   string   `positional-arg-name:"go" description:"Location of go command" required:"true"`
			Args []string `positional-arg-name:"args" description:"Arguments to 'go tool compile'" required:"true"`
		} `positional-args:"true" required:"true"`
	} `command:"compile" description:"Compiles a Go library."`

	Test struct {
	} `command:"test" description:"Templates a test main file."`
}

func main() {
	parser := cli.ParseFlagsOrDie("please_go_tool", "7.6.0", &opts)
	cli.InitLogging(opts.Verbosity)

	if parser.Active.Name == "compile" {
		if err := gotool.LinkPackages(opts.TmpDir); err != nil {
			log.Fatalf("%s", err)
		}
		if opts.Compile.Cover {
			if err := gotool.AnnotateCoverage(opts.Compile.Args.Go, opts.Sources); err != nil {
				log.Fatalf("%s", err)
			}
		}
		// Invoke go tool compile to do its thing.
		args := []string{
			opts.Compile.Args.Go, "tool", "compile",
			"-trimpath", opts.TmpDir,
			"-pack",
			"-o", opts.Compile.Out,
		}
		args = append(args, opts.Compile.Args.Args...)
		args = append(args, opts.Sources...)
		if err := syscall.Exec(opts.Compile.Args.Go, args, os.Environ()); err != nil {
			log.Fatalf("Failed to exec %s: %s", opts.Compile.Args.Go, err)
		}
	}
	/*
		coverVars, err := buildgo.FindCoverVars(opts.Dir, opts.Exclude)
		if err != nil {
			log.Fatalf("Error scanning for coverage: %s", err)
		}
		if err = buildgo.WriteTestMain(opts.Package, buildgo.IsVersion18(opts.Args.Go), opts.Args.Sources, opts.Output, coverVars); err != nil {
			log.Fatalf("Error writing test main: %s", err)
		}
	*/

}
