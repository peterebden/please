package vfs

import (
	"io/ioutil"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/fuse"
)

// A file represents a single file within a filesystem.
type file struct {
	Path     string // Real path on disk
	Writable bool
}

// Info returns an os.FileInfo structure for this file.
func (f file) Info() (os.FileInfo, fuse.Status) {
	info, err := os.Stat(f.Path)
	return info, fuse.ToStatus(err)
}

// DirEntries returns the directory entries for this file.
func (f file) DirEntries() ([]fuse.DirEntry, fuse.Status) {
	// TODO(peterebden): ReadDir is presumably inefficient since it needs to stat() every file.
	//                   Use something that calls through to getdents directly.
	entries, err := ioutil.ReadDir(f.Path)
	fuseEntries := make([]fuse.DirEntry, len(entries))
	for i, entry := range entries {
		fuseEntries[i] = fuse.DirEntry{
			Mode: uint32(entry.Mode()),
			Name: entry.Name(),
			Ino:  entry.Sys().(*syscall.Stat_t).Ino,
		}
	}
	return fuseEntries, fuse.ToStatus(err)
}
