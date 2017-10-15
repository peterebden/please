// Package main implements a remote test worker server for Please that receives
// information about a test, runs it and sends back the results.
package main

import (
	"cli"
	"tools/please_test_worker/worker"
)

var opts = struct {
	Usage      string
	Verbosity  int          `short:"v" long:"verbose" default:"2" description:"Verbosity of output (higher number = more output)"`
	Port       int          `short:"p" long:"port" default:"7792" description:"Port to serve on"`
	MaxMsgSize cli.ByteSize `long:"max_msg_size" default:"500M" description:"Maximum size of message we will accept"`
}{
	Usage: `
please_test_worker is an implementation of Please's remote test worker protocol.
It receives a tests's files and metadata, runs the test itself and returns the
results and any coverage information.

The intention is that one could run a fleet of these to expand the available resources
beyond that available on a single machine. As yet it remains untested at scale.
`,
}

func main() {
	cli.ParseFlagsOrDie("please_test_worker", "7.7.0", &opts)
	cli.InitLogging(opts.Verbosity)
	worker.ServeGrpcForever(opts.Port, int(opts.MaxMsgSize))
}
