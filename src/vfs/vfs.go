// Package vfs implements a virtual file system layer that's used
// for our various temporary directories.
// This allows us to present a virtual mapping which doesn't require
// placing actual physical files on disk.
package vfs

import (
	"os"
	"path"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"gopkg.in/op/go-logging.v1"
)

var log = logging.MustGetLogger("vfs")

// A file represents a single file within a filesystem.
type file struct {
	Path     string // Real path on disk
	Info     os.FileInfo
	Writable bool
	// If it's a directory, this is the list of entries in it.
	Dir []fuse.DirInfo
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
	AddFile(realPath, virtualPath string)
	// Stop closes this filesystem once we're done with it.
	Stop()
}

// New creates a new filesystem and starts it serving at the given path.
func New(root string) (Filesystem, error) {
	fs := &filesystem{
		Root:  root,
		files: map[string]file{},
	}
	// Enable ClientInodes so hard links work
	pnfs := pathfs.NewPathNodeFs(finalFs, &pathfs.PathNodeFsOptions{ClientInodes: *enableLinks})
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

// AddFile adds a new file to this filesystem.
func (fs *filesystem) AddFile(realPath, virtualPath string) {
	fs.files[virtualPath] = file{Path: realPath}
}

// Stop unmounts and stops this filesystem.
func (fs *filesystem) Stop() {
	if err := fs.server.Unmount(); err != nil {
		log.Warning("Failed to unmount VFS: %s", err)
	}
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

// ensureInfo takes a file and guarantees its Info member is populated.
func (fs *filesystem) ensureInfo(f file, s fuse.Status) (file, fuse.Status) {
	if s == fuse.OK && f.Info == nil {
		info, err := os.Stat(f.Path)
		if err != nil {
			return f, fuse.ToStatus(err)
		}
		f.Info = info
		// TODO(peterebden): Consider using *file instead, then we'd not need this update.
		fs.mutex.Lock()
		defer fs.mutex.Unlock()
		fs.files[name] = f
	}
	return f, s
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
	info, err := os.Stat(filename)
	if err != nil {
		return file{}, nil, fuse.ToStatus(err)
	}
	f2 := file{
		Path:     filename,
		Info:     info,
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
	f, s := fs.ensureInfo(fs.getFile(name))
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
	f, s := fs.ensureInfo(fs.getWritableFile(name))
	if s != fuse.OK {
		return s
	} else if !f.Info.IsDir() {
		return fuse.EINVAL
	} else if err := os.Rmdir(f.Path); err != nil {
		return fuse.ToStatus(err)
	}
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	delete(fs.files, name)
	return fuse.OK
}

func (fs *filesystem) Unlink(name string, context *fuse.Context) fuse.Status {
	f, s := fs.ensureInfo(fs.getWritableFile(name))
	if s != fuse.OK {
		return s
	} else if f.Info.IsDir() {
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
func (fs *filesystem) OnMount(nodeFs *PathNodeFs) {
}

func (fs *filesystem) OnUnmount() {
}

func (fs *filesystem) Open(name string, flags uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	if flags & OS.WRONLY {
		return fs.Create(name, flags, 0644, context)
	} else if flags & os.RDWR {
		return nil, fuse.ENOSYS // Not sure if we will need to support this or not
	}
	f, s := fs.getFile(name)
	if s != fuse.OK {
		return nil, s
	}
	f2, err := os.OpenFile(f.Path, os.RDONLY, 0644)
	if err != nil {
		return nil, fuse.ToStatus(err)
	}
	return nodefs.NewLoopbackFile(f2), fuse.OK
}

func (fs *filesystem) Create(name string, flags uint32, mode uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	_, f2, s := fs.getOrCreateFile(name, mode)
	if s != fuse.OK {
		return nil, s
	}
	return nodefs.NewLoopbackFile(f2), fuse.OK
}

func (fs *filesystem) OpenDir(name string, context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	f, s := fs.getFile(name)
	if s != fuse.OK {
		return nil, s
	} else if !f.Info.IsDir() {
		return fuse.ENOTDIR
	}
	return f.Dir
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
	info, err := os.Stat(dest)
	if err != nil {
		return fuse.ToStatus(err)
	}
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.files[linkName] = file{
		Path:     dest,
		Info:     info,
		Writable: true,
	}
	return fuse.OK
}

func (fs *filesystem) Readlink(name string, context *fuse.Context) (string, fuse.Status) {
	f, s := fs.getFile(name)
	if s != fuse.OK {
		return s
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
