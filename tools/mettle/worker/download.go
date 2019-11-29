package worker

import (
	"context"
	"fmt"
	"os"
	"time"

	pb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
)

const timeout = 30 * time.Second

// downloadDirectory downloads & writes out a single Directory proto and all its children.
// TODO(peterebden): can we replace some of this with GetTree, or otherwise share with src/remote?
func (w *worker) downloadDirectory(root string, dir *pb.Directory) error {
	if err := os.MkdirAll(root, os.ModeDir|0775); err != nil {
		return err
	}
	for _, file := range dir.Files {
		filename := path.Join(root, file.Name)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if _, err := w.client.ReadBlobToFile(ctx, file.Digest, filename); err != nil {
			return fmt.Errorf("Failed to download file: %s", err)
		} else if file.IsExecutable {
			if err := os.Chmod(filename, 0755); err != nil {
				return fmt.Errorf("Failed to chmod file: %s", err)
			}
		}
	}
	for _, dir := range dir.Directories {
		d := &pb.Directory{}
		name := path.Join(root, dir.Name)
		if err := w.readProto(dir.Digest, d); err != nil {
			return fmt.Errorf("Failed to download directory metadata for %s: %s", name, err)
		} else if err := w.downloadDirectory(name, d); err != nil {
			return fmt.Errorf("Failed to download directory %s: %s", name, err)
		}
	}
	for _, sym := range dir.Symlinks {
		if err := os.Symlink(sym.Target, path.Join(root, sym.Name)); err != nil {
			return err
		}
	}
	return nil
}

// readProto reads a protobuf from the remote CAS.
// TODO(peterebden): replace with w.client.ReadProto once merged upstream.
func (w *worker) readProto(digest *pb.Digest, msg proto.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	bytes, err := c.client.ReadBlob(ctx, digest)
	if err != nil {
		return err
	}
	return proto.Unmarshal(bytes, msg)
}

// wrap wraps a grpc error in an additional description, but retains its code.
func wrap(err error, msg string, args ...interface{}) error {
	s, ok := grpcstatus.FromError(err)
	if !ok {
		return fmt.Errorf(fmt.Sprintf(msg, args...) + ": " + err.Error())
	}
	return grpcstatus.Errorf(s.Code(), fmt.Sprintf(msg, args...)+": "+s.Message())
}
