// Package fsclient implements a client for the as-yet-unnamed remote artifact filesystem.
package fsclient

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"grpcutil"
	pb "remote/proto/fs"
)

var log = logging.MustGetLogger("fsclient")

// NewClient creates and returns a new client based on the given URL touchpoints that it can
// initialise from.
// N.B. Errors are not returned here since the initialisation is done asynchronously.
func NewClient(urls []string) (*Client, error) {
	// Try the URLs one by one until we find one that works.
}


// A Client is the client to the remote storage pool.
type Client struct {
	nodes []node
	initOnce sync.Once
}

// init ensures the client is initialised.
func (c *Client) init() {
	var e error
	for _, url := range urls {
		client := pb.NewRemoteFSClient(grpcutil.Dial(url))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, err := client.Info(ctx, &pb.InfoRequest{})
		if err != nil {
			multierror.Append(e, err)
			continue
		}
		for
	}

}
