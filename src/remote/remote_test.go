package remote

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/bazelbuild/remote-apis/build/bazel/semver"
	"github.com/thought-machine/please/src/core"
)

func TestInit(t *testing.T) {
	c := newClient()
	assert.NoError(t, c.CheckInitialised())
}

func TestBadAPIVersion(t *testing.T) {
	defer server.Reset()
	server.HighApiVersion.Major = 1
	server.LowApiVersion.Major = 1
	c := newClient()
	assert.Error(t, c.CheckInitialised())
	assert.Contains(t, c.CheckInitialised().Error(), "1.0.0 - 1.1.0")
}

func TestUnsupportedDigest(t *testing.T) {
	defer server.Reset()
	server.DigestFunction = []pb.DigestFunction_Value{
		pb.DigestFunction_SHA256,
		pb.DigestFunction_SHA384,
		pb.DigestFunction_SHA512,
	}
	c := newClient()
	assert.Error(t, c.CheckInitialised())
}

func newClient() *Client {
	state := core.NewDefaultBuildState()
	state.Config.Remote.URL = "127.0.0.1:9987"
	return New(state)
}

// A capsServer implements the server interface for the Capabilities service.
type capsServer struct {
	DigestFunction                []pb.DigestFunction_Value
	LowApiVersion, HighApiVersion semver.SemVer
}

func (s *capsServer) GetCapabilities(ctx context.Context, req *pb.GetCapabilitiesRequest) (*pb.ServerCapabilities, error) {
	return &pb.ServerCapabilities{
		CacheCapabilities: &pb.CacheCapabilities{
			DigestFunction: s.DigestFunction,
			ActionCacheUpdateCapabilities: &pb.ActionCacheUpdateCapabilities{
				UpdateEnabled: true,
			},
			MaxBatchTotalSizeBytes: 4096,
		},
		LowApiVersion:  &s.LowApiVersion,
		HighApiVersion: &s.HighApiVersion,
	}, nil
}

func (s *capsServer) Reset() {
	s.DigestFunction = []pb.DigestFunction_Value{
		pb.DigestFunction_SHA1,
		pb.DigestFunction_SHA256,
	}
	s.LowApiVersion = semver.SemVer{Major: 2}
	s.HighApiVersion = semver.SemVer{Major: 2, Minor: 1}
}

var server = &capsServer{}

func TestMain(m *testing.M) {
	server.Reset()
	lis, err := net.Listen("tcp", ":9987")
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", lis.Addr(), err)
	}
	s := grpc.NewServer()
	pb.RegisterCapabilitiesServer(s, server)
	go s.Serve(lis)
	code := m.Run()
	s.Stop()
	os.Exit(code)
}
