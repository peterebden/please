// Package storage provides the storage backend for elan.
package storage

import (
	"encoding/json"
	"math"
	"os"
	"path"

	"gopkg.in/op/go-logging.v1"
)

var log = logging.MustGetLogger("storage")

const configFile = ".config"

const dirPermissions = os.ModeDir | 0775

// Init initialises the storage backend.
// The given params are used as defaults - it will try to read information existing if
// there is anything there (which is usually the case after initialising & joining the cluster)
func Init(replicas, tokens int, dir string, maxSize uint64) (Storage, error) {
	// Make sure the directory exists
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		log.Error("Failed to create storage directory: %s", err)
		return nil, err
	}
	s := &storage{
		Dir:      dir,
		Replicas: replicas,
		MaxSize:  maxSize,
	}
	// Initialise the tokens to some default values. These will get overwritten later if needed.
	step := uint64(math.MaxUint64 / uint64(tokens))
	s.Data.Tokens = make([]uint64, tokens)
	for i := 0; i < tokens; i++ {
		s.Data.Tokens[i] = step * uint64(i)
	}
	if err := s.LoadConfig(); err != nil && !os.IsNotExist(err) {
		log.Error("Failed to load existing config file: %s", err)
		return nil, err
	}
	// TODO(peterebden): read everything at this point & work out what's there.
	return s, nil
}

type Storage interface {
	// Tokens returns the current set of tokens for this server.
	Tokens() []uint64
	// SetTokens updates the set of tokens.
	SetTokens(tokens []uint64)
	// Shutdown shuts down this storage instance.
	Shutdown()
	// TODO(peterebden): actually useful save/load file functions
}

type storage struct {
	Dir      string
	Replicas int
	MaxSize  uint64
	// Data gets serialised into the config file; it contains the important information
	// that we need to be able to restore on startup.
	Data struct {
		Tokens []uint64 `json:"tokens"`
	}
}

// LoadConfig reloads this node's config from storage.
func (s *storage) LoadConfig() error {
	f, err := os.Open(path.Join(s.Dir, configFile))
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(&s.Data)
}

// SaveConfig saves this node's config for restart.
func (s *storage) SaveConfig() error {
	f, err := os.Create(path.Join(s.Dir, configFile))
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	return enc.Encode(&s.Data)
}

func (s *storage) Tokens() []uint64 {
	return s.Data.Tokens
}

func (s *storage) SetTokens(tokens []uint64) {
	s.Data.Tokens = tokens
}

func (s *storage) Shutdown() {
	if err := s.SaveConfig(); err != nil {
		log.Error("Failed to save config: %s", err)
	}
}
