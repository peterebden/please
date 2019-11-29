// Package main implements an execution server for the Remote Execution API.
package main

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/cli"
	"github.com/thought-machine/please/tools/mettle/api"
	"github.com/thought-machine/please/tools/mettle/common"
	"github.com/thought-machine/please/tools/mettle/worker"
)

var log = logging.MustGetLogger("mettle")

var opts = struct {
	Usage         string
	Verbosity     cli.Verbosity `short:"v" long:"verbosity" default:"notice" description:"Verbosity of output (higher number = more output)"`
	RequestQueue  string        `short:"q" long:"request_queue" required:"true" description:"URL defining the pub/sub queue to connect to for sending requests, e.g. gcppubsub://my-request-queue"`
	ResponseQueue string        `short:"r" long:"response_queue" required:"true" description:"URL defining the pub/sub queue to connect to for sending responses, e.g. gcppubsub://my-response-queue"`
	Storage       string        `short:"s" long:"storage" required:"true" description:"URL to connect to the CAS server on, e.g. localhost:7878"`
	MetricsPort   int           `short:"m" long:"metrics_port" description:"Port to serve Prometheus metrics on"`
	API           struct {
		Port int `short:"p" long:"port" default:"7778" description:"Port to serve on"`
	} `command:"api" description:"Start as an API server"`
	Worker struct {
		Dir string `short:"d" long:"dir" default:"." description:"Directory to run actions in"`
	} `command:"worker" description:"Start as a worker"`
	Dual struct {
		Port int `short:"p" long:"port" default:"7778" description:"Port to serve on"`
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
supported for this is using GCP Cloud Pub/Sub via a gcppubsub:// URL. The queues and
subscriptions must currently be set up manually.
For testing purposes it can be configured in a dual setup where it performs both
roles and in this setup it can be set up with an in-memory queue using a mem:// URL.
Needless to say, this mode does not synchronise with any other servers.

The specific usage of pubsub bears some note; both worker and api servers have long-running
stateful operations and hence should not be casually restarted. In the case of the worker
it will stop receiving new requests when sent a signal, but should be waited for its
current operation to complete before being terminated. Unfortunately the length of time
required is outside of our control (since timeouts on actions are set by clients) so we
cannot give a hard limit of time required.
For the master, it has no persistent storage, but all servers register to receive all
events. Hence when a server restarts it will not have knowledge of all currently running
jobs until an update is sent for each; the suggestion here is to wait for > 1 minute
(after which each live job should have sent an update, and the new server will know about
it). This is easy to arrange in a managed environment like Kubernetes.
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
		// Must ensure the topics are created ahead of time.
		common.MustOpenTopic(opts.RequestQueue)
		common.MustOpenTopic(opts.ResponseQueue)
		go worker.RunForever(opts.RequestQueue, opts.ResponseQueue, opts.Storage, ".")
		api.ServeForever(opts.Dual.Port, opts.RequestQueue, opts.ResponseQueue, opts.Storage)
	} else if cmd == "worker" {
		worker.RunForever(opts.RequestQueue, opts.ResponseQueue, opts.Storage, opts.Worker.Dir)
	} else {
		api.ServeForever(opts.API.Port, opts.RequestQueue, opts.ResponseQueue, opts.Storage)
	}
}
