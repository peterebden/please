package remote

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/thought-machine/please/src/core"
)

// PrintArtifacts prints all the artifacts we use as a .tsv
func (c *Client) PrintArtifacts() {
	type artifact struct{
		Name, Hash string
		Size, Count int
	}

	log.Notice("Calculating targets...")
	targets := map[core.BuildLabel][]*artifact{}
	artifacts := map[string]*artifact{}
	arts := []*artifact{}
	for _, target := range c.state.Graph.AllTargets() {
		if target.State() >= core.Built {
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
				a, present := artifacts[out.Digest.Hash]
				if !present {
					a = &artifact{
						Name: out.Path,
						Hash: out.Digest.Hash,
						Size: int(out.Digest.Size),
					}
					artifacts[a.Hash] = a
					arts = append(arts, a)
				}
				targets[target.Label] = append(targets[target.Label], a)
			}
		}
	}
	totalSize := 0
	for _, target := range c.state.Graph.AllTargets() {
		for input := range c.iterInputs(target, false, target.IsFilegroup) {
			if l := input.Label(); l != nil {
				for _, a := range targets[*l] {
					a.Count++
					totalSize = totalSize + a.Size
				}
			}
		}
	}
	sort.Slice(arts, func(i, j int) bool { return arts[i].Size * arts[i].Count < arts[j].Size * arts[j].Count })
	f, err := os.Create("plz-out/log/remote_artifacts.csv")
	if err != nil {
		log.Errorf("Failed to write remote artifact outputs: %s", err)
		return
	}
	defer f.Close()
	f.Write([]byte("Name,Hash,Size,Count,Total Size,Cumulative\n"))
	cumul := 0
	for _, a := range arts {
		total := a.Count * a.Size
		cumul = cumul + total
		f.Write([]byte(fmt.Sprintf("%s,%s,%d,%d,%d,%f\n", a.Name, a.Hash, a.Size, a.Count, total, float64(cumul) * 100.0 / float64(totalSize))))
	}
}
