// Package main implements elan, a distributed blob storage server.
package main

import (
	"os"
	"strconv"

	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/cli"
	"github.com/thought-machine/please/tools/elan/cluster"
	"github.com/thought-machine/please/tools/elan/grpc"
	"github.com/thought-machine/please/tools/elan/http"
	"github.com/thought-machine/please/tools/elan/storage"
)

var log = logging.MustGetLogger("elan")

var opts = struct {
	Usage     string
	Verbosity cli.Verbosity `short:"v" long:"verbosity" default:"notice" description:"Verbosity of output (higher number = more output)"`

	Network struct {
		DiagnosticPort int      `long:"diagnostic_port" default:"9946" description:"Port to serve HTTP diagnostics on"`
		Port           int      `short:"p" long:"port" default:"9945" description:"Port to communicate on"`
		GossipPort     int      `long:"gossip_port" default:"9944" description:"Port for intra-cluster communication"`
		Peers          []string `long:"peer" required:"true" description:"URLs to discover peers on"`
		Addr           string   `long:"addr" description:"Address to advertise on"`
	} `group:"Options controlling networking & communication"`

	Replication struct {
		Name     string `short:"n" long:"name" env:"ELAN_NAME" description:"Name identifying this server"`
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

func defaultToHostname(s *string, port int) {
	if *s == "" {
		*s, _ = os.Hostname()
		if port != 0 {
			*s += ":" + strconv.Itoa(port)
		}
	}
}

func main() {
	cli.ParseFlagsOrDie("elan", &opts)
	cli.InitLogging(opts.Verbosity)
	defaultToHostname(&opts.Network.Addr, opts.Network.Port)
	defaultToHostname(&opts.Replication.Name, 0)
	s, err := storage.Init(opts.Storage.Dir, uint64(opts.Storage.MaxSize))
	if err != nil {
		log.Fatalf("Failed to initialise storage backend: %s", err)
	}
	config := s.MustLoadConfig(opts.Replication.Name, opts.Network.Addr)
	ring := cluster.NewRing()
	srv := grpc.Start(opts.Network.Port, ring, config, s, opts.Replication.Replicas)
	c, err := cluster.Connect(ring, config, opts.Network.GossipPort, opts.Network.Peers)
	if err != nil {
		log.Fatalf("Failed to join cluster: %s", err)
	} else if err := s.SaveConfig(config); err != nil {
		log.Fatalf("Failed to write updated config: %s", err)
	}
	go srv.ListenUpdates(c.Updates())
	http.ServeForever(opts.Network.DiagnosticPort, srv)
}
