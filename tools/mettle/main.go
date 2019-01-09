// Package main implements a remote test worker server.
package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/cli"
	"github.com/thought-machine/please/src/remote/fsclient"
	"github.com/thought-machine/please/tools/mettle/master"
	"github.com/thought-machine/please/tools/mettle/worker"
)

var log = logging.MustGetLogger("mettle")

var opts = struct {
	Usage     string
	Verbosity cli.Verbosity `short:"v" long:"verbosity" default:"notice" description:"Verbosity of output (higher number = more output)"`

	Master struct {
		Port        int          `short:"p" long:"port" default:"9922" description:"Port to serve on"`
		MetricsPort int          `long:"metrics_port" default:"13434" description:"Port to serve Prometheus metrics on"`
		Retries     int          `short:"r" long:"retries" default:"3" description:"Number of times to retry when all workers are busy"`
		Wait        cli.Duration `short:"d" long:"retry_duration" default:"5s" description:"Wait time between retrying when workers are busy"`
	} `command:"master" description:"Starts this server as the master"`

	Worker struct {
		Master string   `short:"m" long:"master" required:"true" description:"URL of the master to connect to"`
		Name   string   `short:"n" long:"name" description:"Name of this worker instance."`
		Dir    string   `short:"d" long:"dir" default:"." description:"Working directory to run tests in"`
		FSURL  []string `short:"u" long:"fs_url" required:"true" description:"URL of remote FS server"`
	} `command:"worker" description:"Starts this server as a worker"`
}{
	Usage: `
mettle implements a remote test worker for Please.

It can be started in one of two modes; either as the master or as the worker. Typically
one has a pool of workers and a single master; the workers connect to the master as they
start up and register themselves, the master then hands them out to clients on request.
`,
}

func main() {
	command := cli.ParseFlagsOrDie("mettle", &opts)
	cli.InitLogging(opts.Verbosity)
	if command == "master" {
		log.Notice("Starting as a master")
		log.Notice("Serving metrics on http://127.0.0.1:%d/metrics", opts.Master.MetricsPort)
		go func() {
			http.Handle("/metrics", promhttp.Handler())
			log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", opts.Master.MetricsPort), nil))
		}()
		master.Start(opts.Master.Port, opts.Master.Retries, time.Duration(opts.Master.Wait))
	} else {
		log.Notice("Connecting to remote filesystem at %s", strings.Join(opts.Worker.FSURL, ", "))
		client := fsclient.New(opts.Worker.FSURL)
		log.Notice("Starting as worker %s, connecting to master at %s", opts.Worker.Name, opts.Worker.Master)
		worker.Connect(opts.Worker.Master, opts.Worker.Name, opts.Worker.Dir, client)
	}
}
