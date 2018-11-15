// Package main implements elan, a distributed blob storage server.
package main

import (
	"gopkg.in/op/go-logging.v1"

	"cli"
	"tools/elan/cluster"
)

var opts = struct {
	Usage     string
	Verbosity cli.Verbosity `short:"v" long:"verbosity" default:"notice" description:"Verbosity of output (higher number = more output)"`

	Network struct {
		DiscoveryPort int       `long:"discovery_port" default:"9944" description:"Port to communicate on for gossip & discovery"`
		Port          int       `short:"p" long:"port" default:"9945" description:"Port to communicate on for data exchange"`
		URLs          []cli.URL `short:"u" long:"url" required:"true" description:"URLs to discover peers on"`
	} `group:"Options controlling networking & communication"`
}{
	Usage: `
elan is a distributed replicated blob storage server.


`,
}

func main() {
	command := cli.ParseFlagsOrDie("elan", "13.2.5", &opts)
	cli.InitLogging(opts.Verbosity)
	cluster.MustConnect(opts.Network.URLs, opts.Network.DiscoveryPort)
}
