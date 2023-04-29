package main

import (
	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/remote"
)

// New returns a new instance of the remote execution client
func New(state *core.BuildState) core.RemoteClient {
	return remote.New(state)
}

// Required for a plugin but not used.
func main() {}
