package remote

import (
	"fmt"
	"sort"
	
	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	
	"github.com/thought-machine/please/src/core"
)

// PrintArtifacts prints all the artifacts we use as a .tsv
func (c *Client) PrintArtifacts(labels []core.BuildLabel) {
	artifacts, err := c.remoteArtifacts(labels)
	if err != nil {
		log.Fatalf("Failed to calculate artifacts: %s", err)
	}
	fmt.Printf("%s\n", artifacts)
}

type artifact struct{
	Name, Hash string
	Size, Count int
}

func (c *Client) remoteArtifacts(labels []core.BuildLabel) ([]artifact, error) {
	log.Notice("Calculating targets...")
	targets := make([]*core.BuildTarget, len(labels))
	for i, l := range labels {
		targets[i] = c.state.Graph.TargetOrDie(l)
	}
	// Roughly order targets in order of dependencies.
	sort.Slice(targets, func(i, j int) bool { return targets[j].HasDependency(targets[i].Label) })

	artifacts := map[string]*artifact{}
	addDir := func(dir *pb.Directory) {
		for _, file := range dir.Files {
			art, present := artifacts[file.Digest.Hash]
			if !present {
				art = &artifact{
					Name: file.Name,
					Hash: file.Digest.Hash,
					Size: int(file.Digest.SizeBytes),
				}
				artifacts[file.Digest.Hash] = art
			}
			art.Count++
		}
	}
	log.Notice("Finding inputs...")
	for _, t := range targets {
		b, err := c.uploadInputDir(nil, t, false)
		if err != nil {
			return nil, err
		}
		tree := b.Tree("")
		addDir(tree.Root)
		for _, child := range tree.Children {
			addDir(child)
		}
	}
	arts := make([]artifact, 0, len(artifacts))
	for _, a := range artifacts {
		arts = append(arts, *a)
	}
	sort.Slice(arts, func(i, j int) bool { return arts[i].Size * arts[i].Count < arts[j].Size * arts[j].Count })
	return arts, nil
}





