// +build !bootstrap

// Package remote is responsible for communication with remote build servers.
// Some of the nomenclature can be a little confusing since we use "remote" in other contexts
// (e.g. local worker servers that are "remote" to this process). Eventually we might clean
// that all up a bit to be a bit more consistent.
package remote

import (
	"context"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"

	"core"
	pb "test/proto/remote"
)

// Build causes a target to be built on a remote worker.
//
// N.B. It does *not* necessarily cause outputs to appear locally.
func Build(tid int, state *core.BuildState, target *core.BuildTarget) error {

}
