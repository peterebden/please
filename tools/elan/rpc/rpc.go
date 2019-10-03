// Package rpc implements the gRPC server for Elan.
// This contains implementations of the ContentAddressableStorage and ActionCache
// services, but not Execution (even by proxy).
package rpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"sync"

	// Necessary to register providers that we'll use.
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/memblob"

	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/bazelbuild/remote-apis/build/bazel/semver"
	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"gocloud.dev/blob"
	bs "google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/op/go-logging.v1"
)

// ServeForever serves on the given port until terminated.
func ServeForever(port int, storage string) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", lis.Addr(), err)
	}
	srv := &server{}
	s := grpc.NewServer(grpc_recovery.UnaryServerInterceptor(), grpc_recovery.StreamServerInterceptor())
	pb.RegisterCapabilitiesServer(s, server)
	pb.RegisterActionCacheServer(s, server)
	pb.RegisterContentAddressableStorageServer(s, server)
	bs.RegisterByteStreamServer(s, server)
	err = s.Serve(lis)
	log.Fatalf("%s", err)
}

type server struct {
	bucket *blob.Bucket
}

func (s *server) GetCapabilities(ctx context.Context, req *pb.GetCapabilitiesRequest) (*pb.ServerCapabilities, error) {
	return &pb.ServerCapabilities{
		CacheCapabilities: &pb.CacheCapabilities{
			DigestFunction: []pb.DigestFunction_Value{
				pb.DigestFunction_SHA1,
				pb.DigestFunction_SHA256,
			},
			ActionCacheUpdateCapabilities: &pb.ActionCacheUpdateCapabilities{
				UpdateEnabled: true,
			},
			MaxBatchTotalSizeBytes: 4012000, // 4000 Kelly-Bootle standard units
		},
		LowApiVersion:  &semver.SemVer{Major: 2, Minor: 0},
		HighApiVersion: &semver.SemVer{Major: 2, Minor: 0},
	}, nil
}

func (s *server) GetActionResult(ctx context.Context, req *pb.GetActionResultRequest) (*pb.ActionResult, error) {
	ar := &pb.ActionResult{}
	if err := s.readBlobIntoMessage(ctx, req.ActionDigest, ar); err != nil {
		return nil, err
	}
	if req.InlineStdout && ar.StdoutDigest != nil {
		b, err := s.readAllBlob(ctx, ar.StdoutDigest)
		ar.StdoutRaw = b
		return ar, err
	}
	return ar, nil
}

func (s *server) UpdateActionResult(ctx context.Context, req *pb.UpdateActionResultRequest) (*pb.ActionResult, error) {
	return req.ActionResult, s.writeMessage(ctx, req.ActionDigest, req.ActionResult)
}

func (s *server) FindMissingBlobs(ctx context.Context, req *pb.FindMissingBlobsRequest) (*pb.FindMissingBlobsResponse, error) {
	resp := &pb.FindMissingBlobsResponse{}
	var wg sync.WaitGroup
	wg.Add(len(req.BlobDigests))
	var mutex sync.Mutex
	for _, d := range req.BlobDigests {
		go func() {
			if exists, _ := s.bucket.Exists(ctx, s.key(d)); !exists {
				mutex.Lock()
				resp.MissingBlobDigests = append(resp.MissingBlobDigests, d)
				mutex.Unlock()
			}
			wg.Done()
		}()
	}
	return resp, nil
}

func (

func (s *server) readBlob(ctx context.Context, digest *pb.Digest) (io.ReadCloser, error) {
	r, err := s.bucket.NewReader(ctx, s.key(digest), nil)
	if err != nil {
		return status.Errorf(codes.NotFound, "%s", err)
	}
	return r
}

func (s *server) readAllBlob(ctx context.Context, digest *pb.Digest) ([]byte, error) {
	r, err := s.readBlob(ctx, digest)
	if err != nil {
		return err
	}
	defer r.Close()
	return ioutil.ReadAll(r)
}

func (s *server) readBlobIntoMessage(ctx context.Context, digest *pb.Digest, message proto.Message) error {
	if b, err := s.readAllBlob(ctx, digest); err != nil {
		return err
	} else if err := proto.Unmarshal(b, message); err != nil {
		return status.Errorf(codes.Unknown, "%s", err)
	}
	return nil
}

func (s *server) writeBlob(ctx context.Context, digest *pb.Digest, r io.Reader) error {
	w, err := s.bucket.NewWriter(ctx, s.key(digest), nil)
	if err != nil {
		return err
	} else if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return w.Close()
}

func (s *server) writeMessage(ctx context.Context, digest *pb.Digest, message proto.Message) error {
	b, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	return s.writeBlob(ctx, digest, bytes.NewReader(b))
}

func (s *server) key(digest *pb.Digest) string {
	return fmt.Sprintf("%c/%c/%s", digest.Hash[0], digest.Hash[1], digest.Hash)
}
