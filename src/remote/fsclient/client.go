// Package fsclient implements a client for the as-yet-unnamed remote artifact filesystem.
package fsclient

import (
	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"grpcutil"
)

var log = logging.MustGetLogger("fsclient")

// NewClient creates and returns a new client based on the given URL touchpoints that it can
// initialise from.
func NewClient(urls []string) (*Client, error) {

}

// MustNewClient is like NewClient but dies on errors.
func MustNewClient(urls []string) *Client {
	c, err := NewClient(urls)
	if err != nil {
		log.Fatalf("Failed to connect to remote FS storage: %s", err)
	}
	return c
}
