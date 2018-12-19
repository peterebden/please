// +build !bootstrap

// Remote cache based on the distributed remote storage system.
// This probably obsoletes the RPC cache - it has many similar qualities but is
// effectively a more powerful & streamlined design of the same thing.

package cache

import (
	"io"
	"os"

	"golang.org/x/sync/errgroup"

	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/remote/fsclient"
)

func newRemoteFSCache(urls []string) *remoteFSCache {
	return &remoteFSCache{client: fsclient.New(urls)}
}

type remoteFSCache struct {
	client fsclient.Client
}

func (c *remoteFSCache) Store(target *core.BuildTarget, key []byte, files ...string) {
	err := c.store(key, cacheArtifacts(target, files...))
	c.error("Failed to store artifacts with remote server: %s", err)
}

func (c *remoteFSCache) StoreExtra(target *core.BuildTarget, key []byte, file string) {
	err := c.store(key, []string{file})
	c.error("Failed to store artifact with remote server: %s", err)
}

func (c *remoteFSCache) Retrieve(target *core.BuildTarget, key []byte) bool {
	// N.B. this does not support storing / retrieving additional outs correctly.
	//      That doesn't look easy to support through the current API but given its
	//      current narrow usage we might just drop it instead.w
	err := c.retrieve(target, key, cacheArtifacts(target))
	return c.error("Failed to retrieve artifacts from remote server: %s", err)
}

func (c *remoteFSCache) RetrieveExtra(target *core.BuildTarget, key []byte, file string) bool {
	err := c.retrieve(target, key, []string{file})
	return c.error("Failed to retrieve artifact from remote server: %s", err)
}

func (c *remoteFSCache) Clean(target *core.BuildTarget) {
	// We never clean it via this interface. Later we will provide some maintenance tools
	// for the server that will allow dropping specific artifacts if needed.
}

func (c *remoteFSCache) CleanAll() {}

func (c *remoteFSCache) Shutdown() {}

func (c *remoteFSCache) store(key []byte, filenames []string) error {
	contents := make([]io.ReadSeeker, len(filenames))
	for i, filename := range filenames {
		f, err := os.Open(filename)
		if err != nil {
			return err
		}
		contents[i] = f
		defer f.Close()
	}
	return c.client.Put(filenames, key, contents)
}

func (c *remoteFSCache) retrieve(target *core.BuildTarget, key []byte, filenames []string) error {
	var g errgroup.Group
	rs, err := c.client.Get(filenames, key)
	if err != nil {
		return err
	}
	for i, filename := range filenames {
		r := rs[i]
		filename := filename
		g.Go(func() error {
			f, err := os.Open(filename)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(f, r)
			return err
		})
	}
	return g.Wait()
}

func (c *remoteFSCache) error(msg string, err error) bool {
	if err == nil {
		return true
	} else if !fsclient.IsNotFound(err) {
		log.Warning(msg, err)
	}
	return false
}
