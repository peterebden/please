// Package main implements a CAS storage server for the Remote Execution API.
package main

import (
	"github.com/thought-machine/please/src/cli"
	"github.com/thought-machine/please/tools/elan/rpc"
)

var opts = struct {
	Usage     string
	Verbosity cli.Verbosity `short:"v" long:"verbosity" default:"warning" description:"Verbosity of output (higher number = more output)"`
	Port      int           `short:"p" long:"port" default:"7777" description:"Port to serve on"`
	Storage   string        `short:"s" long:"storage" required:"true" description:"URL defining where to store data, eg. gs://bucket-name."`
}{
	Usage: `
Elan is an implementation of the content-addressable storage and action cache services
of the Remote Execution API.

It is fairly simple and assumes that it will be backed by a distributed reliable storage
system (e.g. GCS or S3). Optionally it can be configured to use local file storage (or
in-memory if you enjoy living dangerously) but will not do any sharding, replication or
cleanup - these modes are intended for testing only.
`,
}

func main() {
	cli.ParseFlagsOrDie("Elan", &opts)
	cli.InitLogging(opts.Verbosity)
	rpc.ServeForever(opts.Port, opts.Storage)
}
