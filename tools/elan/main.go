// Package main implements elan, a distributed blob storage server.
package main

import (
	"gopkg.in/op/go-logging.v1"

	"cli"
	"tools/elan/cluster"
	"tools/elan/storage"
)

var log = logging.MustGetLogger("elan")

var opts = struct {
	Usage     string
	Verbosity cli.Verbosity `short:"v" long:"verbosity" default:"notice" description:"Verbosity of output (higher number = more output)"`

	Network struct {
		DiscoveryPort int      `long:"discovery_port" default:"9944" description:"Port to communicate on for gossip & discovery"`
		Port          int      `short:"p" long:"port" default:"9945" description:"Port to communicate on for data exchange"`
		Peers         []string `long:"peer" required:"true" description:"URLs to discover peers on"`
		Addr          string   `long:"addr" description:"Address to advertise on"`
	} `group:"Options controlling networking & communication"`

	Replication struct {
		Name     string `short:"n" long:"name" description:"Friendly name for this server"`
		Replicas int    `long:"replicas" default:"3" description:"Number of replicas for each artifact"`
		Tokens   int    `long:"tokens" default:"10" description:"Number of hash tokens for this node"`
	} `group:"Options controlling replication information"`

	Storage struct {
		Dir     string       `short:"d" long:"dir" required:"true" description:"Directory to store files in"`
		MaxSize cli.ByteSize `short:"m" long:"max_size" default:"50G" description:"Maximum size of files to store"`
	} `group:"Options controlling storage of data"`
}{
	Usage: `
elan is a distributed replicated blob storage server.

Please uses it for storing remote files & communicating them to mettle, its remote worker farm.
`,
}

func main() {
	command := cli.ParseFlagsOrDie("elan", "13.2.5", &opts)
	cli.InitLogging(opts.Verbosity)
	s, err := storage.Init(opts.Replication.Replicas, opts.Replication.Tokens, opts.Storage.Dir, uint64(opts.Storage.MaxSize))
	if err != nil {
		log.Fatalf("Failed to initialise storage backend: %s", err)
	}

	c, err := cluster.Connect(opts.Network.DiscoveryPort, opts.Network.Port, opts.Replication.Name, opts.Network.Addr)
	if err != nil {
		log.Fatalf("Failed to initialise cluster: %s", err)
	}

	if err := c.Join(opts.Network.URLs); err != nil {
		log.Fatalf("Failed to join cluster: %s", err)
	}

}
