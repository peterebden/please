// Package main implements a remote server for managing test workers.
package main

import (
	"gopkg.in/op/go-logging.v1"

	"cli"
	"tools/remote_server/master"
	"tools/remote_server/worker"
)

var log = logging.MustGetLogger("remote_server")

var opts = struct {
	Usage     string
	Verbosity cli.Verbosity `short:"v" long:"verbosity" default:"warning" description:"Verbosity of output (higher number = more output)"`
	Port      int           `short:"p" long:"port" default:"9922" description:"Port to serve on"`

	Master struct {
	} `command:"master" description:"Starts this server as the master"`

	Worker struct {
		Master cli.URL `short:"m" long:"master" required:"true" description:"URL of the master to connect to"`
		Name   string  `short:"n" long:"name" description:"Name of this worker instance."`
		URL    cli.URL `short:"u" long:"url" required:"true" description:"Public URL of this instance"`
		Dir    string  `short:"d" long:"dir" default:"." description:"Working directory to run tests in"`
	} `command:"worker" description:"Starts this server as a worker"`
}{
	Usage: `
remote_server implements a remote test server for Please.

It can be started in one of two modes; either as the master or as the worker. Typically
one has a pool of workers and a single master; the workers connect to the master as they
start up and register themselves, the master then hands them out to clients on request.
`,
}

func main() {
	command := cli.ParseFlagsOrDie("remote_server", "13.2.0", &opts)
	if command == "master" {
		log.Notice("Starting as a master")
		master.Start(opts.Port)
	} else {
		log.Notice("Starting as a worker, connecting to master at %s", opts.Worker.Master)
		worker.Connect(opts.Worker.Master.String(), opts.Worker.Name, opts.Worker.URL.String(), opts.Port, opts.Worker.Dir)
	}
}
