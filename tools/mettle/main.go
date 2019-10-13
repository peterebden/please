// Package main implements an execution server for the Remote Execution API.
package main

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/cli"
	"github.com/thought-machine/please/tools/mettle/api"
	"github.com/thought-machine/please/tools/mettle/worker"
)

var log = logging.MustGetLogger("mettle")

var opts = struct {
	Usage       string
	Verbosity   cli.Verbosity `short:"v" long:"verbosity" default:"notice" description:"Verbosity of output (higher number = more output)"`
	Queue       string        `short:"q" long:"queue" required:"true" description:"URL defining the pub/sub queue to connect to, e.g. gcppubsub://my-queue"`
	Storage     string        `short:"s" long:"storage" required:"true" description:"URL to connect to the CAS server on, e.g. localhost:7878"`
	MetricsPort int           `short:"m" long:"metrics_port" description:"Port to serve Prometheus metrics on"`
	API         struct {
		Port int `short:"p" long:"port" default:"7777" description:"Port to serve on"`
	} `command:"api" description:"Start as an API server"`
	Worker struct {
	} `command:"worker" description:"Start as a worker"`
	Dual struct {
		Port int `short:"p" long:"port" default:"7777" description:"Port to serve on"`
	} `command:"dual" description:"Start as both API server and worker. For local testing only."`
}{
	Usage: `
Mettle is an implementation of the execution service of the Remote Execution API.
It does not implement the storage APIs itself; it must be given the location of another
server to provide that.

It can be configured in one of two modes; either as an API server or a worker. The
API server provides the gRPC API that others contact to request execution of tasks.
Meanwhile the workers perform the actual execution of tasks.

The two server types communicate via a pub/sub queue. The only configuration usefully
supported for this is using GCP Cloud Pub/Sub via a gcppubsub:// URL. For testing
purposes it can be configured in a dual setup where it performs both roles and in this
setup it can be set up with an in-memory queue using a mem:// URL.
`,
}

func main() {
	cmd := cli.ParseFlagsOrDie("Mettle", &opts)
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
	if cmd == "dual" {
		go worker.RunForever(opts.Queue, opts.Storage)
		api.ServeForever(opts.Dual.Port, opts.Queue, opts.Storage)
	} else if cmd == "worker" {
		worker.RunForever(opts.Queue, opts.Storage)
	} else {
		api.ServeForever(opts.API.Port, opts.Queue, opts.Storage)
	}
}
