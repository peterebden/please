// Package vfs implements a virtual file system layer that's used
// for our various temporary directories.
// This allows us to present a virtual mapping which doesn't require
// placing actual physical files on disk.
package vfs

import (
	"os"
	"path"
	"sync"

	"github.com/hanwen/go-fuse"
)

// A file represents a single file within a filesystem.
type file struct {
	Path     string
	Info     os.FileInfo
	Writable bool
}

// A filesystem is the implementation of a fuse.FileSystem.
type filesystem struct {
	Root  string
	files map[string]file
	// Guards access to the above
	mutex sync.RWMutex
}

func (fs *filesystem) getFile(name string) (file, fuse.Status) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()
	if f, present := fs.files[name]; present {
		return &f, fuse.OK
	}
	return nil, fuse.ENOENT
}

func (fs *filesystem) getWritableFile(name string) (file, fuse.Status) {
	if f, s := fs.getFile(name); s != fuse.OK {
		return f, s
	} else if f.Writable {
		return f, s
	}
	return nil, fuse.EROFS
}

func (fs *filesystem) String() string {
	return "vfs rooted at " + fs.Root
}

func (fs *filesystem) SetDebug(debug bool) {}

func (fs *filesystem) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	f, s := fs.getFile(name)
	if s != fuse.OK {
		return s
	}
	return fuse.ToAttr(f.Info), fuse.OK
}

func (fs *filesystem) Chmod(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	f, s := fs.getWritableFile(name)
	if s != fuse.OK {
		return s
	}
	return fuse.ToStatus(os.Chmod(f.Path, mode))
}

func (fs *filesystem) Chown(name string, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	f, s := fs.getWritableFile(name)
	if s != fuse.OK {
		return s
	}
	return fuse.ToStatus(os.Chown(f.Path, uid, gid))
}

func (fs *filesystem) Utimens(name string, Atime *time.Time, Mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	return fuse.OK // Silently pretend success
}

func (fs *filesystem) Truncate(name string, size uint64, context *fuse.Context) (code fuse.Status) {
	f, s := fs.getWritableFile(name)
	if s != fuse.OK {
		return s
	}
	return fuse.ToStatus(os.Truncate(name, size))
}

func (fs *filesystem) Access(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS // Not sure what we are meant to be doing here?
}

func (fs *filesystem) Link(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	f, s := fs.getFile(oldName)
	if s != fuse.OK {
		return s
	} else if _, s := fs.getFile(newName); s == fuse.OK {
		return fuse.EACCES // file exists - not sure if this is the right code
	}
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.files[newName] = f
	return fuse.OK
}

func (fs *filesystem) Mkdir(name string, mode uint32, context *fuse.Context) fuse.Status {
	fullPath := path.Join(fs.Root, name)
	if _, s := fs.getFile(name); s == fuse.OK {
		return fuse.EACCES
	} else if err := os.Mkdir(fullPath); err != nil {
		return fuse.ToStatus(err)
	}
	// TODO(peterebden): This is a bit wasteful; we could implement os.FileInfo ourselves
	//                   and drop it in here instead.
	info, err := os.Stat(fullPath)
	if err != nil {
		return fuse.ToStatus(err)
	}
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.files[name] = file{
		Path:     fullPath,
		Writable: true,
		Info:     info,
	}
	return fuse.OK
}

func (fs *filesystem) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (fs *filesystem) Rename(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	// TODO(peterebden): Need to handle getFile here and back it by a real move.
	f, s := fs.getWritableFile(oldName)
	if s != fuse.OK {
		return s
	}
	fs.files[newName] = f
	delete(fs.files, oldName)
	return fuse.OK
}

func (fs *filesystem) Rmdir(name string, context *fuse.Context) (code fuse.Status)
func (fs *filesystem) Unlink(name string, context *fuse.Context) (code fuse.Status)

// Extended attributes.
func (fs *filesystem) GetXAttr(name string, attribute string, context *fuse.Context) (data []byte, code fuse.Status)
func (fs *filesystem) ListXAttr(name string, context *fuse.Context) (attributes []string, code fuse.Status)
func (fs *filesystem) RemoveXAttr(name string, attr string, context *fuse.Context) fuse.Status
func (fs *filesystem) SetXAttr(name string, attr string, data []byte, flags int, context *fuse.Context) fuse.Status

// Called after mount.
func (fs *filesystem) OnMount(nodeFs *PathNodeFs)
func (fs *filesystem) OnUnmount()

// File handling.  If opening for writing, the file's mtime
// should be updated too.
func (fs *filesystem) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status)
func (fs *filesystem) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, code fuse.Status)

// Directory handling
func (fs *filesystem) OpenDir(name string, context *fuse.Context) (stream []fuse.DirEntry, code fuse.Status)

// Symlinks.
func (fs *filesystem) Symlink(value string, linkName string, context *fuse.Context) (code fuse.Status)
func (fs *filesystem) Readlink(name string, context *fuse.Context) (string, fuse.Status)

func (fs *filesystem) StatFs(name string) *fuse.StatfsOut
