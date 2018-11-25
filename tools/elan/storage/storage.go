// Package storage provides the storage backend for elan.
//
// TODO: - Track total size & clean when appropriate
//       - Better concurrent reading & writing (i.e. ability to read partially written data)
package storage

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/pkg/xattr"
	"gopkg.in/op/go-logging.v1"

	"fs"
	cpb "tools/elan/proto/cluster"
)

var log = logging.MustGetLogger("storage")

const configFile = ".config"

const dirPermissions = os.ModeDir | 0775

const xattrName = "user.elan_atime"

// Init initialises the storage backend.
// The given params are used as defaults - it will try to read information existing if
// there is anything there (which is usually the case after initialising & joining the cluster)
func Init(dir string, maxSize uint64) (Storage, error) {
	// Make sure the directory exists
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		log.Error("Failed to create storage directory: %s", err)
		return nil, err
	}
	s := &storage{
		Dir:     dir,
		MaxSize: maxSize,
		files:   map[file]*fileInfo{},
	}
	return s, fs.Walk(dir, s.walk)
}

type Storage interface {
	// LoadConfig loads the current configuration for this server.
	LoadConfig() (*cpb.Config, error)
	// SaveConfig saves the given config for this cluster.
	SaveConfig(*cpb.Config) error
	// Shutdown shuts down this storage instance.
	Shutdown()
	// Load loads a single file.
	Load(hash uint64, name string) (io.ReadCloser, error)
	// Save saves a single file.
	Save(hash uint64, name string) (io.WriteCloser, error)
}

type file struct {
	Name string
	Hash uint64
}

type fileInfo struct {
	Path    string
	Size    int64
	Atime   int64
	Writing chan struct{}
}

// UpdateAtime sets the atime on the structure, and on the file (which we use an xattr
// for, which has various useful benefits over using the filesystem attribute).
func (info *fileInfo) UpdateAtime() error {
	info.Atime = time.Now().Unix()
	buf := [8]byte{}
	binary.LittleEndian.PutUint64(buf[:], uint64(info.Atime))
	return xattr.LSet(info.Path, xattrName, buf[:])
}

// ReadAtime reads the atime back off a file. The struct is updated with the new value.
func (info *fileInfo) ReadAtime() error {
	b, err := xattr.LGet(info.Path, xattrName)
	if err != nil {
		return err
	}
	info.Atime = int64(binary.LittleEndian.Uint64(b))
	return nil
}

type storage struct {
	Dir     string
	MaxSize uint64
	files   map[file]*fileInfo
	mutex   sync.Mutex
}

// LoadConfig reloads this node's config from storage.
func (s *storage) LoadConfig() (*cpb.Config, error) {
	c := &cpb.Config{}
	f, err := os.Open(path.Join(s.Dir, configFile))
	if err != nil {
		if os.IsNotExist(err) {
			// Config file is allowed not to exist.
			return c, nil
		}
		return nil, err
	}
	defer f.Close()
	return c, jsonpb.Unmarshal(f, c)
}

// SaveConfig saves this node's config for restart.
func (s *storage) SaveConfig(config *cpb.Config) error {
	f, err := os.Create(path.Join(s.Dir, configFile))
	if err != nil {
		return err
	}
	defer f.Close()
	m := jsonpb.Marshaler{Indent: "    "}
	return m.Marshal(f, config)
}

func (s *storage) Shutdown() {
}

func (s *storage) Load(hash uint64, name string) (io.ReadCloser, error) {
	s.mutex.Lock()
	info := s.files[file{Hash: hash, Name: name}]
	s.mutex.Unlock()
	if info == nil {
		return nil, os.ErrNotExist
	} else if info.Writing != nil {
		// TODO(peterebden): We don't have any way of detecting error conditions on the
		//                   write here. Really that should get propagated somehow.
		<-info.Writing
	}
	info.UpdateAtime()
	return os.Open(info.Path)
}

func (s *storage) Save(hash uint64, name string) (io.WriteCloser, error) {
	key := file{Hash: hash, Name: name}
	s.mutex.Lock()
	if s.files[key] != nil {
		return nil, os.ErrExist
	}
	file := &fileInfo{
		Path:    path.Join(s.Dir, name, fmt.Sprintf("%016x", hash)),
		Writing: make(chan struct{}),
	}
	s.files[key] = file
	s.mutex.Unlock()
	if err := os.MkdirAll(path.Dir(info.Path), 0755); err != nil {
		return nil, err
	}
	f, err := os.Create(info.Path)
	if err != nil {
		return nil, err
	}
	return &writeCloser{f: f, info: info}, nil
}

// walk is a function appropriate for fs.Walk for visiting a file.
// Most of the errors are not fatal, we try to read as much of the cache as possible.
func (s *storage) walk(name string, isDir bool) error {
	if isDir {
		return nil
	}
	dir, filename := path.Split(name)
	k := file{Name: strings.Trim(dir[len(s.Dir):], "/")}
	v := &fileInfo{Path: name}
	if dir == "" || len(filename) != 16 {
		// not a cache file, they are always 16 char hex names.
	} else if _, err := fmt.Sscanf(filename, "%016x", &k.Hash); err != nil {
		log.Warning("Invalid name for cache file %s: %s", filename, err)
	} else if err := v.ReadAtime(); err != nil {
		log.Warning("Cache file %s missing %s xattr: %s", name, xattrName, err)
	} else if info, err := os.Lstat(name); err != nil {
		log.Error("Can't stat file %s: %s", name, err)
	} else {
		v.Size = info.Size()
		s.files[k] = v
	}
	return nil
}

// A writeCloser wraps up a file with some extra closing logic.
type writeCloser struct {
	f    *os.File
	info *fileInfo
}

func (w *writeCloser) Write(b []byte) (int, error) {
	n, err := w.f.Write(b)
	w.info.Size += int64(n)
	return n, err
}

func (w *writeCloser) Close() error {
	close(w.info.Writing)
	w.info.Writing = nil
	w.info.UpdateAtime()
	return w.f.Close()
}
