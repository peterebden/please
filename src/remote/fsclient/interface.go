// Package fsclient implements a client for the as-yet-unnamed remote artifact filesystem.
package fsclient

import "io"

// A Client is the client to the as-yet-unnamed remote artifact filesystem.
type Client interface {
	// Get requests a set of files from the remote.
	// It returns a parallel list of readers for them, which are always of the same length
	// as the requested filenames (as long as there is no error).
	Get(filenames []string, hash []byte) ([]io.Reader, error)
	// Put dispatches one or more files to the remote.
	Put(filenames []string, hash []byte, contents []io.ReadSeeker) error
}
