//+build nobootstrap

// Package remote implements support for remote build actions; your build can be
// dispatched to a remote worker to be actioned. This package handles all the
// communication with it via gRPC.
package remote

import (
	"google.golang.org/grpc"
	"net"

	"core"
	pb "remote/proto/remote"
)

// A CallbackFunc is the type of function called on each build request.
// TODO(peterebden): Should probably refactor this out by moving the central build stuff
//                   to a package where this can call it directly.
type CallbackFunc func([]core.BuildLabel, *core.Configuration, bool, bool) (bool, *core.BuildState)

// Serve starts a server and runs it until terminated.
// The given callback function is invoked on each build request.
func Serve(port int, callback CallbackFunc) {

}
