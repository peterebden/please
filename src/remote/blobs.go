package remote

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"

	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	bs "google.golang.org/genproto/googleapis/bytestream"
	// TODO(peterebden): not sure this is my UUID package of choice, but it's the only
	//                   one I have downloaded right now!
	"github.com/hashicorp/go-uuid"
)

// chunkSize is the size of a chunk that we send when using the ByteStream APIs.
const chunkSize = 128 * 1024

// A blob represents something to be uploaded to the remote server.
// It contains the digest of each plus its content or a filename to read it from.
// If the filename is present then the digest's hash may not be populated.
type blob struct {
	Digest pb.Digest
	Data   []byte
	File   string
}

// uploadBlobs uploads a series of blobs to the remote.
// It handles all the logic around the various upload methods etc.
// The given function is a callback that receives a channel to send these blobs on; it
// should close it when finished.
func (c *Client) uploadBlobs(f func(ch chan<- *blob) error) (err error) {
	ch := make(chan *blob, 10) // Buffer it a bit but don't get too far ahead.
	var wg sync.WaitGroup
	go func() {
		err = f(ch)
		wg.Done()
	}()

	reqs := []*pb.BatchUpdateBlobsRequest_Request{}
	var totalSize int64
	for b := range ch {
		if b.Digest.SizeBytes > c.maxBlobBatchSize {
			// This blob individually exceeds the size, have to use this
			// ByteStream malarkey instead.
			if err := c.storeByteStream(b); err != nil {
				return err
			}
		} else if b.Digest.SizeBytes+totalSize > c.maxBlobBatchSize {
			// We have exceeded the total but this blob on its own is OK.
			// Send what we have so far then deal with this one.
			if err := c.sendBlobs(reqs); err != nil {
				return err
			}
			reqs = []*pb.BatchUpdateBlobsRequest_Request{}
			totalSize = 0
		}
		// This file is small enough to be read & stored as part of the reqest.
		// The actual hash might or might not be set.
		if len(b.Digest.Hash) == 0 {
			h, err := c.state.PathHasher.Hash(b.File, false, true)
			if err != nil {
				return err
			}
			b.Digest.Hash = hex.EncodeToString(h)
		}
		// Similarly the data might or might not be available.
		// TODO(peterebden): Unify this into PathHasher somehow so we only read the
		//                   file once (i.e. read it and hash as we go).
		if len(b.Data) == 0 {
			data, err := ioutil.ReadFile(b.File)
			if err != nil {
				return err
			}
			b.Data = data
		}
		reqs = append(reqs, &pb.BatchUpdateBlobsRequest_Request{
			Digest: &b.Digest,
			Data:   b.Data,
		})
	}
	if len(reqs) > 0 {
		if err := c.sendBlobs(reqs); err != nil {
			return err
		}
	}
	wg.Wait()
	return
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
func (c *Client) storeByteStream(b *blob) error {
	// It's probably rare but we might have the contents in memory already at this point.
	// (this shouldn't be a file but could be a serialised proto for example; that's
	// hopefully not common but we do need to handle it here).
	if b.Data != nil {
		return c.reallyStoreByteStream(b, bytes.NewReader(b.Data))
	}
	// Otherwise we need to read the file now.
	f, err := os.Open(b.File)
	if err != nil {
		return err
	}
	defer f.Close()
	return c.reallyStoreByteStream(b, f)
}

func (c *Client) reallyStoreByteStream(b *blob, r io.ReadSeeker) error {
	// The hash might not be set if it's a file.
	// Annoyingly this means we have to read it twice - once to hash it and again to
	// actually upload it :( There isn't any obvious way around this (other than keeping
	// it in memory, but given possibly arbitrarily large files that seems like a bad idea).
	if len(b.Digest.Hash) == 0 {
		h, err := c.state.PathHasher.Hash(b.File, false, true)
		if err != nil {
			return err
		}
		b.Digest.Hash = hex.EncodeToString(h)
	}
	// This is specified in remote_execution.proto, including the UUID.
	id, err := uuid.GenerateUUID()
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%s/uploads/%s/blobs/%s/%d", c.instance, id, b.Digest.Hash, b.Digest.SizeBytes)
	ctx, cancel := context.WithTimeout(context.Background(), reqTimeout)
	defer cancel()
	stream, err := c.bsClient.Write(ctx)
	if err != nil {
		return err
	}
	offset := 0
	buf := make([]byte, chunkSize)
	for {
		n, err := r.Read(buf)
		if err == io.EOF {
			break
		} else if err != nil {
			// TODO(peterebden): How should we indicate failure to the server?
			return err
		} else if err := stream.Send(&bs.WriteRequest{
			ResourceName: name,
			WriteOffset:  int64(offset),
			Data:         buf[:n],
		}); err != nil {
			return err
		}
		offset += n
	}
	if err := stream.Send(&bs.WriteRequest{FinishWrite: true}); err != nil {
		return err
	}
	_, err = stream.CloseAndRecv()
	return err
}
