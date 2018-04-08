package vfs

import (
	"fmt"
	"os"
	"path"

	"core"
)

// Fuse is the a filesystem implementation backed by FUSE.
const Fuse = "fuse"

// File is a filesystem implementation backed by real files.
const File = "file"

// A Filesystem is the public interface to working with the VFS layer.
type Filesystem interface {
	// AddFile adds a file into the system at a particular location.
	// It isn't threadsafe and must only be called before the Filesystem is used.
	AddFile(virtualPath, realPath string) error
	// Stop closes this filesystem once we're done with it.
	Stop()
	// Extract retrieves a file from the virtual filesystem to elsewhere
	// on the real filesystem.
	Extract(virtualPath, realPath string) error
}

// New creates and returns a new filesystem of the given type rooted at the given location.
// Currently supported types are "file" and "fuse".
// If the empty string is passed it will attempt various implementations until it finds one that works.
func New(kind, root string) (Filesystem, error) {
	if kind == Fuse {
		return newVFS(root)
	} else if kind == File {
		return newFS(root)
	} else if kind != "" {
		return nil, fmt.Errorf("Unknown filesystem implementation %s", kind)
	}
	// Try them in order.
	f, err := newVFS(root)
	if err == nil {
		return f, nil
	}
	log.Warning("Cannot initialise FUSE filesystem: %s. Falling back to file-based system.", err)
	return newFS(root)
}

// newFS creates a new file-based filesystem at the given root.
// It delegates all operations down to the real filesystem so lacks many of the features
// of the virtualised FUSE system.
func newFS(root string) (*localFS, error) {
	if err := os.MkdirAll(root, core.DirPermissions); err != nil {
		return nil, err
	}
	return &localFS{Root: root}
}

type localFS struct {
	Root string
}

func (fs *localFS) AddFile(virtualPath, realPath string) error {
	virtualPath = path.Join(fs.Root, virtualPath)
	if err := os.MkdirAll(path.Dir(virtualPath)); err != nil {
		return err
	}
	return core.RecursiveCopyFile(realPath, virtualPath, 0, true, true)
}

func (fs *localFS) Stop() {
	// Clean up after ourselves
	if err := os.RemoveAll(fs.Root); err != nil {
		log.Warning("Failed to clean filesystem: %s", err)
	}
}

func (fs *localFS) Extract(virtualPath, realPath string) error {
	return core.RecursiveCopyFile(path.Join(fs.Root, virtualPath), realPath, 0, true, true)
}
