// Package main implements a standalone parser binary,
// which is simply a benchmark for how fast we can read a large number
// of BUILD files.
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alecthomas/participle/lexer"
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

const (
	// ANSI formatting codes
	reset     = "\033[0m"
	boldRed   = "\033[31;1m"
	boldWhite = "\033[37;1m"
	red       = "\033[31m"
	white     = "\033[37m"
)

// printErrorMessage prints a detailed error message for a lexer error.
// Not quite sure why it's a lexer error not a parser error, but not to worry.
func printErrorMessage(err *lexer.Error, filename string) bool {
	// -1's follow for 0-indexing
	if line := readLine(filename, err.Pos.Line-1); line != "" {
		charsBefore := err.Pos.Column - 1
		if charsBefore < 0 { // strings.Repeat panics if negative
			charsBefore = 0
		}
		fmt.Printf("%s%s%s:%s%d%s:%s%d%s: %serror:%s %s%s%s\n%s%s%s%c%s%s\n%s^%s\n",
			boldWhite, filename, reset,
			boldWhite, err.Pos.Line, reset,
			boldWhite, err.Pos.Column, reset,
			boldRed, reset,
			boldWhite, err.Message, reset,
			white, line[:charsBefore],
			red, line[charsBefore],
			white, line[charsBefore+1:],
			strings.Repeat(" ", charsBefore), reset,
		)
		return true
	}
	return false
}

// readLine reads a file and returns a particular line of it.
func readLine(filename string, line int) string {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return ""
	}
	lines := bytes.Split(b, []byte{'\n'})
	if len(lines) <= line {
		return ""
	}
	return string(lines[line])
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
					atomic.AddInt64(&errors, 1)
					if lerr, ok := err.(*lexer.Error); ok {
						if printErrorMessage(lerr, file) {
							continue
						}
					}
					log.Error("Error parsing %s: %s", file, err)
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
