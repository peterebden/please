// Package cluster provides node discovery & clustering for elan.
package cluster

import (
	"bytes"
	"fmt"
	stdlog "log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/memberlist"
	"gopkg.in/op/go-logging.v1"
)

var log = logging.MustGetLogger("cluster")

// A Cluster represents a connection to the other nodes in the cluster.
type Cluster interface {
	// Join connects to the cluster using the given URLs for discovery.
	Join(urls []string) error
	// Shutdown shuts down this cluster node.
	Shutdown()
	// Nodes returns the RPC URLs of nodes currently known in the cluster.
	Nodes() []string
}

// Start starts the cluster server. It does not join the cluster - call Join
// on the returned cluster for that.
func Connect(port, rpcPort int, name, addr string) (Cluster, error) {
	c := memberlist.DefaultLANConfig()
	c.BindPort = port
	c.AdvertisePort = port
	c.Delegate = &delegate{url: addr + strconv.Itoa(rpcPort)}
	c.Logger = stdlog.New(&logWriter{}, "", 0)
	c.AdvertiseAddr = defaultToHostname(addr)
	c.Name = defaultToHostname(name)
	list, err := memberlist.Create(c)
	if err != nil {
		return nil, err
	}
	n := list.LocalNode()
	log.Notice("Memberlist initi1alised, this node is %s / %s:%d", n.Name, n.Addr, port)
	return &cluster{list: list}, nil
}

func defaultToHostname(s string) string {
	if s == "" {
		if hostname, err := os.Hostname(); err == nil {
			return hostname
		}
	}
	return s
}

type cluster struct {
	list *memberlist.Memberlist
}

func (c *cluster) Join(urls []string) error {
	log.Notice("Joining cluster at %s", strings.Join(urls, ", "))
	n, err := c.list.Join(urls)
	log.Info("Contacted %d nodes", n)
	return err
}

func (c *cluster) Shutdown() {
	log.Warning("Disconnecting from cluster")
	if err := c.list.Leave(2 * time.Second); err != nil {
		log.Error("Error leaving cluster: %s", err)
	}
	if err := c.list.Shutdown(); err != nil {
		log.Error("Error shutting down cluster: %s", err)
	}
}

func (c *cluster) Nodes() []string {
	nodes := c.list.Members()
	ret := make([]string, len(nodes))
	for i, n := range nodes {
		ret[i] = string(n.Meta)
	}
	return ret
}

// A delegate is our implementation of memberlist's Delegate interface.
// Somewhat awkwardly we have to implement the whole thing to provide metadata for our node,
// which we only really need to do to communicate our RPC URL.
type delegate struct {
	url string
}

func (d *delegate) NodeMeta(limit int) []byte                  { return []byte(d.url) }
func (d *delegate) NotifyMsg([]byte)                           {}
func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte { return nil }
func (d *delegate) LocalState(join bool) []byte                { return nil }
func (d *delegate) MergeRemoteState(buf []byte, join bool)     {}

// A logWriter is a wrapper around our logger to decode memberlist's prefixes into our logging levels.
type logWriter struct{}

// logLevels maps memberlist's prefixes to our logging levels.
var logLevels = map[string]func(format string, args ...interface{}){
	"[ERR]":   log.Errorf,
	"[ERROR]": log.Errorf,
	"[WARN]":  log.Warning,
	"[INFO]":  log.Info,
	"[DEBUG]": log.Debug,
}

// Write implements the io.Writer interface
func (w *logWriter) Write(b []byte) (int, error) {
	for prefix, f := range logLevels {
		if bytes.HasPrefix(b, []byte(prefix)) {
			f(string(bytes.TrimSpace(bytes.TrimPrefix(b, []byte(prefix)))))
			return len(b), nil
		}
	}
	return 0, fmt.Errorf("Couldn't decide how to log %s", string(b))
}
