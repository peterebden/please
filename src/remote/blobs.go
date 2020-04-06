package remote

import (
	"context"
	"encoding/csv"
	"os"
	"strconv"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/chunker"
	"golang.org/x/sync/errgroup"

	"github.com/thought-machine/please/src/core"
)

// uploadBlobs uploads a series of blobs to the remote.
// It handles all the logic around the various upload methods etc.
// The given function is a callback that receives a channel to send these blobs on; it
// should close it when finished.
func (c *Client) uploadBlobs(f func(ch chan<- *chunker.Chunker) error) error {
	const buffer = 10 // Buffer it a bit but don't get too far ahead.
	ch := make(chan *chunker.Chunker, buffer)
	var g errgroup.Group
	g.Go(func() error { return f(ch) })
	chomks := []*chunker.Chunker{}
	for chomk := range ch {
		chomks = append(chomks, chomk)
	}
	if err := g.Wait(); err != nil {
		return err
	}
	// TODO(peterebden): This timeout is kind of arbitrary since it represents a lot of requests.
	ctx, cancel := context.WithTimeout(context.Background(), 10*c.reqTimeout)
	defer cancel()
	return c.client.UploadIfMissing(ctx, chomks...)
}

// PrintOutputs prints all the outputs from a set of targets as a .csv
func (c *Client) PrintArtifacts(labels []core.BuildLabel) {
	var total int64
	w := csv.NewWriter(os.Stdout)
	for _, l := range labels {
		target := c.state.Graph.TargetOrDie(l)
		command, digest, err := c.buildAction(target, false)
		if err != nil {
			log.Errorf("Error calculating outputs for %s: %s", target, err)
			continue
		}
		_, ar := c.retrieveResults(target, command, digest, false)
		if ar == nil {
			log.Warning("Failed to retrieve results for %s", target)
			continue
		}
		outputs, err := c.client.FlattenActionOutputs(context.Background(), ar)
		if err != nil {
			log.Error("Failed to download outputs for %s: %s", target, err)
			continue
		}
		for _, out := range outputs {
			w.Write([]string{out.Path, out.Digest.Hash, strconv.Itoa(int(out.Digest.Size))})
			total += out.Digest.Size
		}
	}
	w.Flush()
	log.Notice("Total size: %d bytes", total)
}
