// Package main implements a multipurpose helper for compiling Go code.
// This is mostly intended to implement a level of logic that 'go build' would normally do
// without having to implement all of that in bash.
package main

import (
	"os"
	"syscall"

	"github.com/jessevdk/go-flags"
	"gopkg.in/op/go-logging.v1"

	"tools/please_go_tool/gotool"
)

var log = logging.MustGetLogger("plz_go_tool")

var opts = struct {
	Usage     string
	Verbosity int      `short:"v" long:"verbose" default:"1" description:"Verbosity of output (higher number = more output, default 1 -> warnings and errors only)"`
	TmpDir    string   `long:"tmp_dir" env:"TMP_DIR" required:"true" description:"Temp dir that we're running in"`
	Sources   []string `long:"srcs" env:"SRCS" env-delim:" " description:"Source files" required:"true"`
	Out       string   `short:"o" long:"out" env:"OUT" description:"Output file"`
	Package   string   `short:"p" long:"package" description:"Package path" env:"PKG"`
	GoPath    []string `long:"gopath" env:"GOPATH" env-delim:":" description:"GOPATH to search in"`
	OS        string   `long:"os" env:"OS" description:"OS we're compiling for"`
	Arch      string   `long:"arch" env:"ARCH" description:"Architecture we're compiling for"`
	Exclude   []string `short:"x" long:"exclude" default:"third_party/go" description:"Directories to exclude from search"`
	Cover     bool     `short:"c" long:"cover" env:"COVERAGE" description:"Annotate files for coverage"`
	TestMain  bool     `short:"t" long:"test_main" description:"Template a test main file"`
	Go        string   `short:"g" long:"go" description:"Location of go command" required:"true"`
	Args      struct {
		Args []string `positional-arg-name:"args" description:"Arguments to 'go tool compile'"`
	} `positional-args:"true" required:"true"`
}{
	Usage: `
please_go_tool is used by Please for various Go functions.

The two main ones that it handles currently are:
 - Some preamble to invoking 'go tool compile' which is used to ensure various
   files are in the correct places and so forth. This also includes
   templating in coverage variables when using 'plz cover'.
 - Test main templating as 'go test' would normally do. This isn't currently
   available as a standalone function of any builtin Go tools.
`,
}

func main() {
	// Note that we can't use src/cli here because we don't want to introduce more circular dependencies.
	args, err := flags.Parse(&opts)
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	formatter := logging.NewBackendFormatter(backend, logging.MustStringFormatter("%{time:15:04:05.000} %{level:7s}: %{message}"))
	leveled := logging.AddModuleLevel(backend)
	leveled.SetLevel(logging.Level(opts.Verbosity), "")
	logging.SetBackend(leveled, formatter)
	if err != nil {
		log.Fatalf("%s", err)
	} else if len(args) != 0 {
		log.Fatalf("unparsed arguments: %s", args)
	}

	if !opts.TestMain {
		if err := gotool.LinkPackages(opts.TmpDir); err != nil {
			log.Fatalf("%s", err)
		}
		if opts.Cover {
			if err := gotool.AnnotateCoverage(opts.Go, opts.Sources); err != nil {
				log.Fatalf("%s", err)
			}
		}
		// Invoke go tool compile to do its thing.
		args := []string{
			opts.Go, "tool", "compile",
			"-trimpath", opts.TmpDir,
			"-pack",
			"-o", opts.Out,
		}
		for _, p := range opts.GoPath {
			args = append(args, "-I", p, "-I", p+"/pkg/"+opts.OS+"_"+opts.Arch)
		}
		args = append(args, opts.Args.Args...)
		args = append(args, opts.Sources...)
		if err := syscall.Exec(opts.Go, args, os.Environ()); err != nil {
			log.Fatalf("Failed to exec %s: %s", opts.Go, err)
		}
	} else {
		coverVars, err := gotool.FindCoverVars(opts.TmpDir, opts.Exclude, opts.Sources)
		if err != nil {
			log.Fatalf("Error scanning for coverage: %s", err)
		}
		if err = gotool.WriteTestMain(opts.Package, gotool.IsVersion18(opts.Go), opts.Sources, opts.Out, coverVars); err != nil {
			log.Fatalf("Error writing test main: %s", err)
		}
	}
}
