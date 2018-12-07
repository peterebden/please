// +build !bootstrap

// Remote cache based on the distributed remote storage system.
// This probably obsoletes the RPC cache - it has many similar qualities but is
// effectively a more powerful & streamlined design of the same thing.

package cache

import (
	"bytes"
	"io"
	"os"
	"path"

	"remote/fsclient"
)

func newRemoteFSCache(urls []string) *remoteFSCache {
	return &remoteFSCache{client: fsclient.New(urls)}
}

type remoteFSCache struct {
	client fsclient.Client
}

func (c *remoteFSCache) Store(target *BuildTarget, key []byte, files ...string) {
	if err := c.store(key, cacheArtifacts(target, files)); err != nil {
		log.Warning("Failed to store artifacts with remote server: %s", err)
	}
}

func (c *remoteFSCache) StoreExtra(target *BuildTarget, key []byte, file string) {
	if err := c.store(key, []string{file}); err != nil {
		log.Warning("Failed to store artifacts with remote server: %s", err)
	}
}

func (c *remoteFSCache) Retrieve(target *BuildTarget, key []byte) {
	// N.B. this does not support storing / retrieving additional outs correctly.
	//      That doesn't look easy to support through the current API but given its
	//      current narrow usage we might just drop it instead.
	if err := c.retrieve(target, key, cacheArtifacts(target, files)); err != nil {
		log.Warning("Failed to store artifacts with remote server: %s", err)
	}
}

func (c *remoteFSCache) RetrieveExtra(target *BuildTarget, key []byte, file string) {
	if err := c.retrieve(target, key, []string{file}); err != nil {
		log.Warning("Failed to store artifacts with remote server: %s", err)
	}
}

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

func (c *remoteFSCache) retrieve(target *BuildTarget, key []byte, filenames []string) error {
	rs, err := c.client.Get(filenames, key)
	if err != nil {
		return err
	}
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
