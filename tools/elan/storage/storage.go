// Package storage provides the storage backend for elan.
package storage

import (
	"os"
	"path"

	"github.com/golang/protobuf/jsonpb"
	"gopkg.in/op/go-logging.v1"

	cpb "tools/elan/proto/cluster"
)

var log = logging.MustGetLogger("storage")

const configFile = ".config"

const dirPermissions = os.ModeDir | 0775

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
	}
	// TODO(peterebden): read everything at this point & work out what's there.
	return s, nil
}

type Storage interface {
	// LoadConfig loads the current configuration for this server.
	LoadConfig() (*cpb.Config, error)
	// SaveConfig saves the given config for this cluster.
	SaveConfig(*cpb.Config) error
	// Shutdown shuts down this storage instance.
	Shutdown()
	// TODO(peterebden): actually useful save/load file functions
}

type storage struct {
	Dir     string
	MaxSize uint64
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
