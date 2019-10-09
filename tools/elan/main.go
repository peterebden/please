// Package main implements a CAS storage server for the Remote Execution API.
package main

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/cli"
	"github.com/thought-machine/please/tools/elan/rpc"
)

var log = logging.MustGetLogger("elan")

var opts = struct {
	Usage       string
	Verbosity   cli.Verbosity `short:"v" long:"verbosity" default:"notice" description:"Verbosity of output (higher number = more output)"`
	Port        int           `short:"p" long:"port" default:"7777" description:"Port to serve on"`
	Storage     string        `short:"s" long:"storage" required:"true" description:"URL defining where to store data, eg. gs://bucket-name."`
	MetricsPort int           `short:"m" long:"metrics_port" description:"Port to serve Prometheus metrics on"`
}{
	Usage: `
Elan is an implementation of the content-addressable storage and action cache services
of the Remote Execution API.

It is fairly simple and assumes that it will be backed by a distributed reliable storage
system. Currently the only production-ready backend that is supported is GCS.
Optionally it can be configured to use local file storage (or in-memory if you enjoy
living dangerously) but will not do any sharding, replication or cleanup - these
modes are intended for testing only.
`,
}

func main() {
	cli.ParseFlagsOrDie("Elan", &opts)
	cli.InitLogging(opts.Verbosity)
	if opts.MetricsPort != 0 {
		go func() {
			log.Notice("Serving metrics on :%d", opts.MetricsPort)
			http.Handle("/metrics", promhttp.Handler())
			if err := http.ListenAndServe(fmt.Sprintf(":%d", opts.MetricsPort), nil); err != nil {
				log.Fatalf("%s", err)
			}
		}()
	}
	log.Notice("Serving on :%d", opts.Port)
	rpc.ServeForever(opts.Port, opts.Storage)
}
