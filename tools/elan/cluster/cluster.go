// Package cluster provides node discovery & clustering for elan.
package cluster

import (
	"bytes"
	"fmt"
	stdlog "log"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/memberlist"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/grpcutil"
	pb "github.com/thought-machine/please/src/remote/proto/fs"
	cpb "github.com/thought-machine/please/tools/elan/proto/cluster"
)

var log = logging.MustGetLogger("cluster")

// gossipRetries is the number of seconds we wait for gossip to settle down after
// joining the cluster.
const gossipRetries = 60

// gossipTolerance is the length of time we want to wait before seeing the gossip settle.
const gossipTolerance = 5 * time.Second

// A Cluster represents a connection to the other nodes in the cluster.
type Cluster interface {
	// Shutdown shuts down this cluster node.
	Shutdown()
	// Updates returns a channel on which the caller can receive updates about nodes changing.
	Updates() <-chan *pb.Node
}

// Connect starts the cluster server.
func Connect(ring *Ring, config *cpb.Config, port int, peers []string) (Cluster, error) {
	c := memberlist.DefaultLANConfig()
	c.BindPort = port
	c.AdvertisePort = port
	ch := make(chan *pb.Node, 10)
	d := &delegate{
		ring:       ring,
		config:     config,
		ch:         ch,
		lastUpdate: time.Now(),
	}
	c.Delegate = d
	c.Events = d
	c.Logger = stdlog.New(&logWriter{}, "", 0)
	c.Name = config.ThisNode.Name
	list, err := memberlist.Create(c)
	if err != nil {
		return nil, err
	}
	d.list = list
	cl := &cluster{
		list:   list,
		ring:   ring,
		config: config,
		ch:     ch,
	}
	log.Notice("Attempting to contact initial peers...")
	n, err := list.Join(peers)
	log.Notice("Contacted %d nodes", n)
	if n > 0 {
		d.WaitForGossip()
	}
	if len(cl.config.ThisNode.Ranges) == 0 {
		// We don't have any ranges; presumably we are starting for the first time.
		log.Notice("Running first-time initialisation & generating tokens...")
		if err := ring.Add(cl.config.ThisNode); err != nil {
			return nil, err
		} else if err := list.UpdateNode(10 * time.Second); err != nil {
			return nil, err
		}
		config.Initialised = true
		log.Notice("First-time init complete, this node now possesses %d ranges", len(cl.config.ThisNode.Ranges))
	} else {
		// Load in any existing nodes that aren't already in there.
		for _, n := range cl.config.Nodes {
			if ring.Node(n.Name) == nil {
				ring.Update(n)
			}
		}
	}
	// Force an update of all the nodes now (MergeRemoteState seems lazy about it)
	for _, n := range list.Members() {
		d.MergeRemoteState(n.Meta, false)
	}
	log.Notice("Joined cluster")
	grpcutil.AddCleanup(cl.Shutdown)
	return cl, nil
}

type cluster struct {
	list   *memberlist.Memberlist
	ring   *Ring
	config *cpb.Config
	ch     chan *pb.Node
}

func (c *cluster) Shutdown() {
	log.Warning("Disconnecting from cluster")
	if c.ch != nil {
		close(c.ch)
	}
	if err := c.list.Leave(2 * time.Second); err != nil {
		log.Error("Error leaving cluster: %s", err)
	}
	if err := c.list.Shutdown(); err != nil {
		log.Error("Error shutting down cluster: %s", err)
	}
}

func (c *cluster) Updates() <-chan *pb.Node {
	// Feed the channel one of every node initially
	go func() {
		for _, n := range c.ring.Export() {
			c.ch <- n
		}
	}()
	return c.ch
}

// A delegate is our implementation of memberlist's Delegate interface.
type delegate struct {
	ring       *Ring
	config     *cpb.Config
	list       *memberlist.Memberlist
	lastUpdate time.Time
	ch         chan<- *pb.Node
}

func (d *delegate) NodeMeta(limit int) []byte {
	// N.B. we cannot serialise the whole config here because it is typically larger than the limit.
	//      Weirdly in some cases this seems to get sent to MergeRemoteState so we try to still
	//      serialise the same type here.
	b, _ := proto.Marshal(&cpb.Config{ThisNode: d.ring.Node(d.config.ThisNode.Name)})
	return b
}

func (d *delegate) NotifyMsg(buf []byte) {
	// This is a message sent from MergeRemoteState (see below), we are getting updated about
	// our state. Usually this is because we've forgotten it and are rejoining.
	d.MergeRemoteState(buf, false)
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return nil
}

func (d *delegate) LocalState(join bool) []byte {
	b, err := proto.Marshal(&cpb.Config{Nodes: d.ring.Export()})
	if err != nil {
		log.Error("error serialising local state: %s", err)
	}
	return b
}

func (d *delegate) MergeRemoteState(buf []byte, join bool) {
	cfg := &cpb.Config{}
	if err := proto.Unmarshal(buf, cfg); err != nil {
		log.Error("Failed to decode state message: %s", err)
		return
	}
	for _, node := range cfg.Nodes {
		d.updateNode(node)
	}
	if len(cfg.Nodes) == 0 && cfg.ThisNode != nil {
		d.updateNode(cfg.ThisNode)
		log.Info("Got state update from %s, current ring size: %d", cfg.ThisNode.Name, len(d.ring.nodes))
	} else {
		log.Info("Got state update, current ring size: %d", len(d.ring.nodes))
	}
}

// updateNode updates a single node as received from a remote.
func (d *delegate) updateNode(node *pb.Node) {
	changed, err := d.ring.Update(node)
	if err != nil {
		log.Error("Failed to add node to ring: %s", err)
	} else if changed {
		d.ch <- d.ring.Node(node.Name)
		d.lastUpdate = time.Now()
	}
}

// WaitForGossip waits for gossip to settle down after we've joined the cluster.
// Ideally we want to wait until we've got all the messages, although that's hard to tell and
// kind of not 100% required so we just do our best.
func (d *delegate) WaitForGossip() {
	log.Notice("Waiting for gossip to settle down...")
	d.lastUpdate = time.Now()
	for i := 0; i < gossipRetries; i++ {
		time.Sleep(1 * time.Second)
		if time.Since(d.lastUpdate) > gossipTolerance {
			return
		}
	}
	log.Warning("Giving up on waiting for gossip; too much churn?")
}

func (d *delegate) NotifyJoin(node *memberlist.Node) {
	d.notify(node, "joining", true)
	if d.list != nil && node.Name != d.config.ThisNode.Name {
		// Node is joining, give it a download of everything we know right now.
		go func() {
			b, _ := proto.Marshal(&cpb.Config{Nodes: d.ring.Export()})
			if err := d.list.SendReliable(node, b); err != nil {
				log.Warning("Failed to send ranges to %s: %s", node.Name, err)
			}
		}()
	}
}

func (d *delegate) NotifyLeave(node *memberlist.Node) {
	d.notify(node, "leaving", false)
}

func (d *delegate) NotifyUpdate(node *memberlist.Node) {
	d.notify(node, "update", true)
}

func (d *delegate) notify(node *memberlist.Node, action string, alive bool) {
	if d.list != nil && node.Name != d.config.ThisNode.Name {
		log.Notice("Cluster update from %s: %s", node.Name, action)
		if node, changed := d.ring.UpdateNode(node.Name, alive); node != nil && changed {
			d.ch <- node
		}
		if alive {
			d.MergeRemoteState(node.Meta, alive)
		}
	}
}

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
