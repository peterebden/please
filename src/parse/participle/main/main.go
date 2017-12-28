// Package main implements a standalone parser binary,
// which is simply a benchmark for how fast we can read a large number
// of BUILD files.
package main

import (
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/op/go-logging.v1"

	"cli"
	"core"
	"parse/participle"
)

var log = logging.MustGetLogger("parser")

var opts = struct {
	Usage      string
	Verbosity  int `short:"v" long:"verbose" default:"2" description:"Verbosity of output (higher number = more output)"`
	NumThreads int `short:"n" long:"num_threads" default:"10" description:"Number of concurrent parse threads to run"`
	Args       struct {
		BuildFiles []string `positional-arg-name:"files" description:"BUILD files to parse"`
	} `positional-args:"true"`
}{
	Usage: `Test parser for BUILD files using our standalone parser.`,
}

func main() {
	cli.ParseFlagsOrDie("parser", "11.0.0", &opts)
	cli.InitLogging(opts.Verbosity)

	p := participle.NewParser()
	ch := make(chan string, 100)
	var wg sync.WaitGroup
	wg.Add(opts.NumThreads)
	total := len(opts.Args.BuildFiles)
	state := core.NewBuildState(opts.NumThreads, nil, opts.Verbosity, core.DefaultConfiguration())

	start := time.Now()
	var errors int64
	for i := 0; i < opts.NumThreads; i++ {
		go func() {
			for file := range ch {
				pkg := core.NewPackage(file)
				if err := p.ParseFile(state, pkg, file); err != nil {
					log.Error("Error parsing %s: %s", file, err)
					atomic.AddInt64(&errors, 1)
				}
			}
			wg.Done()
		}()
	}

	for _, file := range opts.Args.BuildFiles {
		ch <- file
	}
	close(ch)
	wg.Wait()

	log.Notice("Parsed %d files in %s", total, time.Since(start))
	log.Notice("Success: %d / %d (%0.2f%%)", total-int(errors), total, 100.0*float64(total-int(errors))/float64(total))
}
