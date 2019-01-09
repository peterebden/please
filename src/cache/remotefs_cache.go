// +build !bootstrap

// Remote cache based on the distributed remote storage system.
// This probably obsoletes the RPC cache - it has many similar qualities but is
// effectively a more powerful & streamlined design of the same thing.

package cache

import (
	"path"

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
	err := c.store(key, target, c.cacheArtifacts(target, files...))
	c.error("Failed to store artifacts with remote server: %s", err)
}

func (c *remoteFSCache) StoreExtra(target *core.BuildTarget, key []byte, file string) {
	err := c.store(key, target, []string{file})
	c.error("Failed to store artifact with remote server: %s", err)
}

func (c *remoteFSCache) Retrieve(target *core.BuildTarget, key []byte) bool {
	// N.B. this does not support storing / retrieving additional outs correctly.
	//      That doesn't look easy to support through the current API but given its
	//      current narrow usage we might just drop it instead.
	err := c.retrieve(target, key, c.cacheArtifacts(target))
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

func (c *remoteFSCache) store(key []byte, target *core.BuildTarget, filenames []string) error {
	return c.client.PutRelative(filenames, key, target.OutDir())
}

func (c *remoteFSCache) retrieve(target *core.BuildTarget, key []byte, filenames []string) error {
	return c.client.GetInto(filenames, key, target.OutDir())
}

func (c *remoteFSCache) error(msg string, err error) bool {
	if err == nil {
		return true
	} else if !fsclient.IsNotFound(err) {
		log.Warning(msg, err)
	}
	return false
}

func (c *remoteFSCache) cacheArtifacts(target *core.BuildTarget, files ...string) []string {
	ret := cacheArtifacts(target, files...)
	dir := target.ShortOutDir()
	for i, out := range ret {
		ret[i] = path.Join(dir, out)
	}
	return ret
}
