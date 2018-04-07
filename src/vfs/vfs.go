// Package vfs implements a virtual file system layer that's used
// for our various temporary directories.
// This allows us to present a virtual mapping which doesn't require
// placing actual physical files on disk.
package vfs

import (
	"io/ioutil"
	"os"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"gopkg.in/op/go-logging.v1"
)

var log = logging.MustGetLogger("vfs")

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

// A filesystem is the implementation of a fuse.FileSystem.
// It multiplexes together three systems:
//  - A readonly system of the target's sources & dependencies
//  - A read/write system of the outputs during the build.
//  - Optionally, the contents of the target's output if it's a zipfile.
//    This allows reading from it as though it were a real filesystem.
type filesystem struct {
	Root  string
	files map[string]file
	// Guards access to the above
	mutex  sync.RWMutex
	server *fuse.Server
}

// A Filesystem is the public interface to working with the VFS layer.
type Filesystem interface {
	// AddFile adds a file into the system at a particular location.
	// It isn't threadsafe and must only be called before the Filesystem is used.
	AddFile(virtualPath, realPath string)
	// Stop closes this filesystem once we're done with it.
	Stop()
}

// New creates a new filesystem and starts it serving at the given path.
func New(root string) (Filesystem, error) {
	// Ensure the directory exists
	if err := os.MkdirAll(root, os.ModeDir|0775); err != nil {
		return nil, err
	}
	fs := &filesystem{
		Root:  root,
		files: map[string]file{},
	}
	// Enable ClientInodes so hard links work
	pnfs := pathfs.NewPathNodeFs(fs, &pathfs.PathNodeFsOptions{ClientInodes: true})
	conn := nodefs.NewFileSystemConnector(pnfs.Root(), nodefs.NewOptions())
	server, err := fuse.NewServer(conn.RawFS(), root, &fuse.MountOptions{
		AllowOther:    false,
		Name:          "plzfs",
		FsName:        root,
		DisableXAttrs: true,
	})
	if err != nil {
		return nil, err
	}
	go server.Serve()
	fs.server = server
	return fs, nil
}

// Must is the same as New, but dies if there is any error.
func Must(root string) Filesystem {
	fs, err := New(root)
	if err != nil {
		log.Fatalf("Failed to mount VFS: %s", err)
	}
	return fs
}

// AddFile adds a new file to this filesystem.
func (fs *filesystem) AddFile(virtualPath, realPath string) {
	fs.files[virtualPath] = file{Path: realPath}
}

// Stop unmounts and stops this filesystem.
func (fs *filesystem) Stop() {
	if err := fs.server.Unmount(); err != nil {
		log.Warning("Failed to unmount VFS: %s", err)
	}
}

func (fs *filesystem) getFileOnly(name string) (file, fuse.Status) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()
	if f, present := fs.files[name]; present {
		return f, fuse.OK
	}
	return file{}, fuse.ENOENT
}

func (fs *filesystem) getFile(name string) (file, fuse.Status) {
	if f, s := fs.getFileOnly(name); s != fuse.ENOENT {
		return f, s
	}
	// Directories are lazily discovered. We might need to find its parent.
	if dir := path.Dir(name); dir != "." {
		if f, s := fs.getFile(dir); s == fuse.OK {
			// Register all the files in this dir now.
			entries, s := f.DirEntries()
			if s != fuse.OK {
				return file{}, s
			}
			retf := file{}
			rets := fuse.ENOENT
			fs.mutex.Lock()
			defer fs.mutex.Unlock()
			for _, entry := range entries {
				fname := path.Join(dir, entry.Name)
				f2 := file{
					Path:     path.Join(f.Path, entry.Name),
					Writable: f.Writable,
				}
				fs.files[fname] = f2
				if fname == name {
					retf = f2
					rets = fuse.OK
				}
			}
			return retf, rets
		}
	}
	return file{}, fuse.ENOENT
}

func (fs *filesystem) getWritableFile(name string) (file, fuse.Status) {
	if f, s := fs.getFile(name); s != fuse.OK {
		return f, s
	} else if f.Writable {
		return f, s
	}
	return file{}, fuse.EROFS
}

func (fs *filesystem) getOrCreateFile(name string, perm os.FileMode) (file, *os.File, fuse.Status) {
	if f, s := fs.getWritableFile(name); s != fuse.ENOENT {
		return f, nil, s
	}
	// File not found means it's fine to create a new one.
	filename := path.Join(fs.Root, name)
	f, err := os.OpenFile(filename, os.O_RDWR, perm)
	if err != nil {
		return file{}, nil, fuse.ToStatus(err)
	}
	f2 := file{
		Path:     filename,
		Writable: true,
	}
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.files[name] = f2
	return f2, f, fuse.OK
}

func (fs *filesystem) String() string {
	return "plzfs rooted at " + fs.Root
}

func (fs *filesystem) SetDebug(debug bool) {}

func (fs *filesystem) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	f, s := fs.getFile(name)
	if s != fuse.OK {
		return nil, s
	}
	i, s := f.Info()
	return fuse.ToAttr(i), s
}

func (fs *filesystem) Chmod(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	f, s := fs.getWritableFile(name)
	if s != fuse.OK {
		return s
	}
	return fuse.ToStatus(os.Chmod(f.Path, os.FileMode(mode)))
}

func (fs *filesystem) Chown(name string, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	f, s := fs.getWritableFile(name)
	if s != fuse.OK {
		return s
	}
	return fuse.ToStatus(os.Chown(f.Path, int(uid), int(gid)))
}

func (fs *filesystem) Utimens(name string, Atime *time.Time, Mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	return fuse.OK // Silently pretend success
}

func (fs *filesystem) Truncate(name string, size uint64, context *fuse.Context) (code fuse.Status) {
	f, s := fs.getWritableFile(name)
	if s != fuse.OK {
		return s
	}
	return fuse.ToStatus(os.Truncate(f.Path, int64(size)))
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
	} else if err := os.Mkdir(fullPath, os.FileMode(mode)); err != nil {
		return fuse.ToStatus(err)
	}
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.files[name] = file{
		Path:     fullPath,
		Writable: true,
	}
	return fuse.OK
}

func (fs *filesystem) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (fs *filesystem) Rename(oldName string, newName string, context *fuse.Context) fuse.Status {
	// TODO(peterebden): Need to handle getFile here and back it by a real move.
	f, s := fs.getWritableFile(oldName)
	if s != fuse.OK {
		return s
	}
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.files[newName] = f
	delete(fs.files, oldName)
	return fuse.OK
}

func (fs *filesystem) Rmdir(name string, context *fuse.Context) fuse.Status {
	f, s := fs.getWritableFile(name)
	if s != fuse.OK {
		return s
	} else if info, s := f.Info(); s != fuse.OK {
		return s
	} else if !info.IsDir() {
		return fuse.EINVAL
	} else if err := os.Remove(f.Path); err != nil {
		return fuse.ToStatus(err)
	}
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	delete(fs.files, name)
	return fuse.OK
}

func (fs *filesystem) Unlink(name string, context *fuse.Context) fuse.Status {
	f, s := fs.getWritableFile(name)
	if s != fuse.OK {
		return s
	} else if info, s := f.Info(); s != fuse.OK {
		return s
	} else if info.IsDir() {
		return fuse.EINVAL
	} else if err := os.Remove(f.Path); err != nil {
		return fuse.ToStatus(err)
	}
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	delete(fs.files, name)
	return fuse.OK
}

// Extended attributes.
func (fs *filesystem) GetXAttr(name string, attribute string, context *fuse.Context) ([]byte, fuse.Status) {
	return nil, fuse.ENOSYS
}

func (fs *filesystem) ListXAttr(name string, context *fuse.Context) ([]string, fuse.Status) {
	return nil, fuse.ENOSYS
}

func (fs *filesystem) RemoveXAttr(name string, attr string, context *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (fs *filesystem) SetXAttr(name string, attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

// Called after mount.
func (fs *filesystem) OnMount(nodeFs *pathfs.PathNodeFs) {
}

func (fs *filesystem) OnUnmount() {
}

func (fs *filesystem) Open(name string, flags uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	if flags&uint32(os.O_WRONLY|os.O_RDWR) != 0 {
		return fs.Create(name, flags, 0644, context)
	}
	f, s := fs.getFile(name)
	if s != fuse.OK {
		return nil, s
	}
	f2, err := os.OpenFile(f.Path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fuse.ToStatus(err)
	}
	return nodefs.NewLoopbackFile(f2), fuse.OK
}

func (fs *filesystem) Create(name string, flags uint32, mode uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	_, f2, s := fs.getOrCreateFile(name, os.FileMode(mode))
	if s != fuse.OK {
		return nil, s
	}
	return nodefs.NewLoopbackFile(f2), fuse.OK
}

func (fs *filesystem) OpenDir(name string, context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	f, s := fs.getFile(name)
	if s != fuse.OK {
		return nil, s
	}
	return f.DirEntries()
}

func (fs *filesystem) Symlink(value string, linkName string, context *fuse.Context) (code fuse.Status) {
	f, s := fs.getFile(value)
	if s != fuse.OK {
		return s
	} else if _, s := fs.getWritableFile(linkName); s == fuse.OK {
		return fuse.EACCES
	} else if s != fuse.ENOENT {
		return s
	}
	dest := path.Join(fs.Root, linkName)
	if err := os.Symlink(f.Path, dest); err != nil {
		return fuse.ToStatus(err)
	}
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.files[linkName] = file{
		Path:     dest,
		Writable: true,
	}
	return fuse.OK
}

func (fs *filesystem) Readlink(name string, context *fuse.Context) (string, fuse.Status) {
	f, s := fs.getFile(name)
	if s != fuse.OK {
		return "", s
	}
	link, err := os.Readlink(f.Path)
	return link, fuse.ToStatus(err)
}

func (fs *filesystem) StatFs(name string) *fuse.StatfsOut {
	s := syscall.Statfs_t{}
	if err := syscall.Statfs(fs.Root, &s); err != nil {
		return nil
	}
	out := &fuse.StatfsOut{}
	out.FromStatfsT(&s)
	return out
}
