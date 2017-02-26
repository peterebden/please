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
	Out       string   `short:"o" long:"out" env:"OUT" description:"Output file"`
	Package   string   `short:"p" long:"package" description:"Package path" env:"PKG"`
	Exclude   []string `short:"x" long:"exclude" default:"third_party/go" description:"Directories to exclude from search"`
	Cover     bool     `short:"c" long:"cover" description:"Annotate files for coverage"`
	TestMain  bool     `short:"t" long:"test_main" description:"Template a test main file"`
	Args      struct {
		Go   string   `positional-arg-name:"go" description:"Location of go command" required:"true"`
		Args []string `positional-arg-name:"args" description:"Arguments to 'go tool compile'"`
	} `positional-args:"true" required:"true"`
}

func main() {
	cli.ParseFlagsOrDie("please_go_tool", "7.6.0", &opts)
	cli.InitLogging(opts.Verbosity)

	if !opts.TestMain {
		if err := gotool.LinkPackages(opts.TmpDir); err != nil {
			log.Fatalf("%s", err)
		}
		if opts.Compile.Cover {
			if err := gotool.AnnotateCoverage(opts.Args.Go, opts.Sources); err != nil {
				log.Fatalf("%s", err)
			}
		}
		// Invoke go tool compile to do its thing.
		args := []string{
			opts.Args.Go, "tool", "compile",
			"-trimpath", opts.TmpDir,
			"-pack",
			"-o", opts.Compile.Out,
		}
		args = append(args, opts.Args.Args...)
		args = append(args, opts.Sources...)
		if err := syscall.Exec(opts.Args.Go, args, os.Environ()); err != nil {
			log.Fatalf("Failed to exec %s: %s", opts.Args.Go, err)
		}
	} else if parser.Active.Name == "test" {
		coverVars, err := gotool.FindCoverVars(opts.TmpDir, opts.Exclude)
		if err != nil {
			log.Fatalf("Error scanning for coverage: %s", err)
		}
		if err = gotool.WriteTestMain(opts.Package, gotool.IsVersion18(opts.Args.Go), opts.Sources, opts.Output, coverVars); err != nil {
			log.Fatalf("Error writing test main: %s", err)
		}
	}
}
