// Package remote provides our interface to the Google remote execution APIs
// (https://github.com/bazelbuild/remote-apis) which Please can use to distribute
// work to remote servers.
package remote

import (
	"github.com/bazelbuild/remote-apis"
	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"
)

var log = logging.MustGetLogger("remote")

// A Client is the interface to the remote API.
type Client struct {
	actionCacheClient remoteexecution.ActionCacheClient
	storageClient     remoteexecution.ContentAddressableStorageClient
}

// New returns a new Client instance.
// It dies on any errors (but these actually rarely happen apart from dire misconfiguration,
// most are returned later when concrete requests are sent).
func New(actionURL, storageURL string) *Client {

}

func mustDial(url string) *grpc.ClientConn {
	conn, err := grpc.Dial()
}
