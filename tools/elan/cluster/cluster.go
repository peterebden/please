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

// Start starts the cluster server.
func Connect(ring *Ring, config *cpb.Config, port int, peers []string) (Cluster, error) {
	c := memberlist.DefaultLANConfig()
	c.BindPort = port
	c.AdvertisePort = port
	ch := make(chan *pb.Node, 10)
	d := &delegate{
		ring: ring,
		node: config.ThisNode,
		ch:   ch,
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
		list: list,
		ring: ring,
		node: d.node,
		ch:   ch,
	}
	n, err := list.Join(peers)
	log.Notice("Contacted %d nodes", n)
	d.WaitForGossip()
	if len(cl.node.Ranges) == 0 {
		// We don't have any ranges; presumably we are starting for the first time.
		log.Notice("Running first-time initialisation & generating tokens...")
		if err := ring.Add(cl.node); err != nil {
			return nil, err
		}
		return cl, list.UpdateNode(10 * time.Second)
	}
	log.Notice("Joined cluster")
	grpcutil.AddCleanup(cl.Shutdown)
	return cl, nil
}

type cluster struct {
	list *memberlist.Memberlist
	ring *Ring
	node *pb.Node
	ch   chan *pb.Node
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
	node       *pb.Node
	list       *memberlist.Memberlist
	lastUpdate time.Time
	ch         chan<- *pb.Node
}

func (d *delegate) NodeMeta(limit int) []byte {
	return nil
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
	b, _ := proto.Marshal(d.node)
	return b
}
func (d *delegate) MergeRemoteState(buf []byte, join bool) {
	node := &pb.Node{}
	if err := proto.Unmarshal(buf, node); err != nil {
		log.Error("Failed to decode state message: %s", err)
	} else if len(node.Ranges) == 0 {
		// Special case: this node has not joined before (or has forgotten itself)
		// so if we know who it is, we tell it so.
		if existing := d.ring.Node(node.Name); existing != nil {
			go func() {
				b, _ := proto.Marshal(existing)
				if n := d.find(node.Name); n != nil {
					log.Notice("Updating node %s with its ranges", node.Name)
					if err := d.list.SendReliable(n, b); err != nil {
						log.Warning("Failed to send ranges to %s: %s", node.Name, err)
					}
				}
			}()
		}
	} else if err := d.ring.Update(node); err != nil {
		log.Error("Failed to add node to ring: %s", err)
	}
	d.lastUpdate = time.Now()
	log.Notice("Got state update from %s", node.Name)
}

// WaitForGossip waits for gossip to settle down after we've joined the cluster.
// Ideally we want to wait until we've got all the messages, although that's hard to tell and
// kind of not 100% required so we just do our best.
func (d *delegate) WaitForGossip() {
	log.Notice("Waiting for gossip to settle down...")
	for i := 0; i < gossipRetries; i++ {
		time.Sleep(1 * time.Second)
		if time.Since(d.lastUpdate) > gossipTolerance {
			return
		}
	}
	log.Warning("Giving up on waiting for gossip; too much churn?")
}

func (d *delegate) find(name string) *memberlist.Node {
	for _, node := range d.list.Members() {
		if node.Name == name {
			return node
		}
	}
	return nil
}

func (d *delegate) NotifyJoin(node *memberlist.Node) {
	log.Notice("Got notification of %s joining the cluster", node.Name)
	if node := d.ring.UpdateNode(node.Name, true); node != nil {
		d.ch <- node
	}
}

func (d *delegate) NotifyLeave(node *memberlist.Node) {
	log.Notice("Got notification of %s leaving the cluster", node.Name)
	if node := d.ring.UpdateNode(node.Name, false); node != nil {
		d.ch <- node
	}
}

func (d *delegate) NotifyUpdate(node *memberlist.Node) {
	log.Notice("Got update from %s", node.Name)
	if node := d.ring.UpdateNode(node.Name, true); node != nil {
		d.ch <- node
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
