// Package remote provides our interface to the Google remote execution APIs
// (https://github.com/bazelbuild/remote-apis) which Please can use to distribute
// work to remote servers.
package remote

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/bazelbuild/remote-apis/build/bazel/semver"
	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/core"
)

var log = logging.MustGetLogger("remote")

// Timeout to initially contact the server.
const dialTimeout = 5 * time.Second

// Timeout for actual requests
const reqTimeout = 2 * time.Minute

// Maximum number of times we retry a request.
const maxRetries = 3

// The API version we support.
var apiVersion = semver.SemVer{Major: 2}

// A Client is the interface to the remote API.
//
// It provides a higher-level interface over the specific RPCs available.
type Client struct {
	actionCacheClient pb.ActionCacheClient
	storageClient     pb.ContentAddressableStorageClient
	initOnce          sync.Once
	state             *core.BuildState
	err               error // for initialisation

	// This is for servers have have multiple instances. Right now we never set it but
	// we keep this here to remind us where it would need to go in the API.
	instance string

	// Server-sent cache properties
	maxBlobBatchSize int64
	cacheWritable    bool

	// Cache this for later
	bashPath string
}

// New returns a new Client instance.
// It begins the process of contacting the remote server but does not wait for it.
func New(state *core.BuildState) *Client {
	c := &Client{state: state}
	go c.CheckInitialised() // Kick off init now, but we don't have to wait for it.
	return c
}

// CheckInitialised checks that the client has connected to the server correctly.
func (c *Client) CheckInitialised() error {
	c.initOnce.Do(c.init)
	return c.err
}

// init is passed to the sync.Once to do the actual initialisation.
func (c *Client) init() {
	c.err = func() error {
		// TODO(peterebden): We may need to add the ability to have multiple URLs which we
		//                   would then query for capabilities to discover which is which.
		conn, err := grpc.Dial(c.state.Config.Remote.URL.String(),
			grpc.WithTimeout(dialTimeout),
			grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(grpc_retry.WithMax(maxRetries))))
		if err != nil {
			return err
		}
		// Query the server for its capabilities. This tells us whether it is capable of
		// execution, caching or both.
		ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
		defer cancel()
		resp, err := pb.NewCapabilitiesClient(conn).GetCapabilities(ctx, &pb.GetCapabilitiesRequest{
			InstanceName: c.instance,
		})
		if err != nil {
			return err
		} else if lessThan(&apiVersion, resp.LowApiVersion) || lessThan(resp.HighApiVersion, &apiVersion) {
			return fmt.Errorf("Unsupported API version; we require %s but server only supports %s - %s", printVer(&apiVersion), printVer(resp.LowApiVersion), printVer(resp.HighApiVersion))
		}
		caps := resp.CacheCapabilities
		if caps == nil {
			return fmt.Errorf("Cache capabilities not supported by server (we do not support execution-only servers)")
		}
		if err := c.chooseDigest(caps.DigestFunction); err != nil {
			return err
		}
		if caps.ActionCacheUpdateCapabilities != nil {
			c.cacheWritable = caps.ActionCacheUpdateCapabilities.UpdateEnabled
		}
		c.maxBlobBatchSize = caps.MaxBatchTotalSizeBytes
		c.actionCacheClient = pb.NewActionCacheClient(conn)
		c.storageClient = pb.NewContentAddressableStorageClient(conn)
		// Look this up just once now.
		bash, err := core.LookBuildPath("bash", c.state.Config)
		c.bashPath = bash
		return err
	}()
	if c.err != nil {
		log.Error("Error setting up remote execution client: %s", c.err)
	}
}

// chooseDigest selects a digest function that we will use.w
func (c *Client) chooseDigest(fns []pb.DigestFunction_Value) error {
	for _, fn := range fns {
		// Right now the only choice we can make generally is SHA1.
		// In future we might let ourselves be guided by this and choose something else
		// that matches the server (but that implies that all targets have to be hashed
		// with it, hence we'd have to synchronously initialise against the server, and
		// it's unclear whether this will be an issue in practice anyway).
		if fn == pb.DigestFunction_SHA1 {
			return nil
		}
	}
	return fmt.Errorf("No acceptable hash function available; server supports %s but we require SHA1", fns)
}

func (c *Client) GetArtifact() {
}

// Store stores a set of artifacts for a single build target.
func (c *Client) Store(target *core.BuildTarget, key []byte, files []string) error {
	// v0.1: just do BatchUpdateBlobs  <-- we are here
	// v0.2: honour the max size to do ByteStreams
	// v0.3: get the action cache involved
	reqs := make([]*pb.BatchUpdateBlobsRequest_Request, 0, len(files))
	ar := &pb.ActionResult{
		// We never cache any failed actions so ExitCode is implicitly 0.
		ExecutionMetadata: &pb.ExecutedActionMetadata{
			Worker: c.state.Config.Remote.Name,
			// TODO(peterebden): Add some kind of temporary metadata so we can know at least
			//                   the start/completed timestamps and stdout/stderr.
			//                   We will need stdout at least for post-build functions.
			OutputUploadStartTimestamp: toTimestamp(time.Now()),
		},
	}
	var totalSize int64
	for i, file := range files {
		// Find out how big the file is upfront, if it's huge we don't want to read it
		// all into RAM at once (we will end up using this ByteStream malarkey instead).
		info, err := os.Lstat(file)
		if err != nil {
			return err
		} else if mode := info.Mode(); mode&os.ModeDir != 0 {
			// It's a directory, needs special treatment
			root, children, err := c.digestDir(file, nil)
			if err != nil {
				return err
			}
			digest := digestMessage(&pb.Tree{
				Root:     root,
				Children: children,
			})
			ar.OutputDirectories = append(ar.OutputDirectories, &pb.OutputDirectory{
				Path:       file,
				TreeDigest: digest,
			})
			continue
		} else if mode&os.ModeSymlink != 0 {
			target, err := os.Readlink(file)
			if err != nil {
				return err
			}
			// TODO(peterebden): Work out if we need to give a shit about
			//                   OutputDirectorySymlinks or not. Seems like we shouldn't
			//                   need to care since symlinks don't know the type of thing
			//                   they point to?
			ar.OutputFileSymlinks = append(ar.OutputFileSymlinks, &pb.OutputSymlink{
				Path:   file,
				Target: target,
			})
			continue
		} else if size := info.Size(); size > c.maxBlobBatchSize {
			// This blob individually exceeds the size, have to stream it.
			h, err := c.storeByteStream(file, size)
			if err != nil {
				return err
			}
			// Still need to save this for later
			ar.OutputFiles = append(ar.OutputFiles, &pb.OutputFile{
				Path: file,
				Digest: &pb.Digest{
					Hash:      hex.EncodeToString(h),
					SizeBytes: size,
				},
				IsExecutable: (info.Mode() & 0111) != 0,
			})
			continue
		} else if size+totalSize > c.maxBlobBatchSize {
			// We have exceeded the total but this blob on its own is OK.
			// Send what we have so far then deal with this one.
			if err := c.sendBlobs(reqs); err != nil {
				return err
			}
			reqs = make([]*pb.BatchUpdateBlobsRequest_Request, 0, len(files)-i)
			totalSize = 0
		}
		req, digest, err := c.digestFile(file, info.Size())
		if err != nil {
			return err
		}
		reqs = append(reqs, req)
		ar.OutputFiles = append(ar.OutputFiles, &pb.OutputFile{
			Path:         file,
			Digest:       digest,
			IsExecutable: (info.Mode() & 0111) != 0,
		})
	}
	if len(reqs) > 0 {
		return c.sendBlobs(reqs)
	}
	// OK, now the blobs are uploaded, we also need to upload the Action itself.
	digest, err := c.uploadAction(target, key)
	if err != nil {
		return err
	}
	// Now we can use that to upload the result itself.
	ctx, cancel := context.WithTimeout(context.Background(), reqTimeout)
	defer cancel()
	_, err = c.actionCacheClient.UpdateActionResult(ctx, &pb.UpdateActionResultRequest{
		InstanceName: c.instance,
		ActionDigest: digest,
		ActionResult: ar,
	})
	return err
}

// sendBlobs dispatches a set of blobs to the remote CAS server.
func (c *Client) sendBlobs(reqs []*pb.BatchUpdateBlobsRequest_Request) error {
	ctx, cancel := context.WithTimeout(context.Background(), reqTimeout)
	defer cancel()
	resp, err := c.storageClient.BatchUpdateBlobs(ctx, &pb.BatchUpdateBlobsRequest{
		InstanceName: c.instance,
		Requests:     reqs,
	})
	if err != nil {
		return err
	}
	// TODO(peterebden): this is not really great handling - we should really use Details
	//                   instead of Message (since this ends up being user-facing) and
	//                   shouldn't just take the first one. This will do for now though.
	for _, r := range resp.Responses {
		if r.Status.Code != 0 {
			return fmt.Errorf("%s", r.Status.Message)
		}
	}
	return nil
}

// storeByteStream sends a single file as a bytestream. This is required when
// it's over the size limit for BatchUpdateBlobs.
// It returns the hash of the stored file.
func (c *Client) storeByteStream(file string, size int64) ([]byte, error) {
	return nil, fmt.Errorf("Unimplemented")
}

// digestFile creates an UpdateBlobsRequest and a Digest from a single file.
// It must be a real file (not a dir or symlink) and must be under the size limit.
func (c *Client) digestFile(file string, size int64) (*pb.BatchUpdateBlobsRequest_Request, *pb.Digest, error) {
	// TODO(peterebden): Unify this into PathHasher somehow so we only read the
	//                   file once (i.e. read it and hash as we go).
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}
	h, err := c.state.PathHasher.Hash(file, false, true)
	if err != nil {
		return nil, nil, err
	}
	digest := &pb.Digest{
		Hash:      hex.EncodeToString(h),
		SizeBytes: size,
	}
	return &pb.BatchUpdateBlobsRequest_Request{
		Digest: digest,
		Data:   b,
	}, digest, nil
}

// digestDir calculates the digest for a directory.
// It returns Directory protos for the directory and all its (recursive) children.
func (c *Client) digestDir(dir string, children []*pb.Directory) (*pb.Directory, []*pb.Directory, error) {
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	// We have to upload all the files as we go along too.
	reqs := make([]*pb.BatchUpdateBlobsRequest_Request, 0, len(entries))
	var totalSize int64
	d := &pb.Directory{}
	for i, entry := range entries {
		name := entry.Name()
		fullname := path.Join(dir, name)
		if mode := entry.Mode(); mode&os.ModeDir != 0 {
			dir, descendants, err := c.digestDir(fullname, children)
			if err != nil {
				return nil, nil, err
			}
			d.Directories = append(d.Directories, &pb.DirectoryNode{
				Name:   name,
				Digest: digestMessage(dir),
			})
			children = append(children, descendants...)
			continue
		} else if mode&os.ModeSymlink != 0 {
			target, err := os.Readlink(fullname)
			if err != nil {
				return nil, nil, err
			}
			d.Symlinks = append(d.Symlinks, &pb.SymlinkNode{
				Name:   name,
				Target: target,
			})
			continue
		} else if size := entry.Size(); size > c.maxBlobBatchSize {
			// This blob individually exceeds the size, have to stream it.
			if _, err := c.storeByteStream(fullname, size); err != nil {
				return nil, nil, err
			}
			continue
		} else if size+totalSize > c.maxBlobBatchSize {
			// We have exceeded the total but this blob on its own is OK.
			// Send what we have so far then deal with this one.
			if err := c.sendBlobs(reqs); err != nil {
				return nil, nil, err
			}
			reqs = make([]*pb.BatchUpdateBlobsRequest_Request, 0, len(entries)-i)
			totalSize = 0
		}
		req, _, err := c.digestFile(fullname, entry.Size())
		if err != nil {
			return nil, nil, err
		}
		reqs = append(reqs, req)
	}
	if len(reqs) > 0 {
		return d, children, c.sendBlobs(reqs)
	}
	return d, children, nil
}
