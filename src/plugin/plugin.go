// Package plugin provides runtime plugin support for some of plz's more esoteric functionality
package plugin

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"plugin"

	"github.com/thought-machine/please/src/cli/logging"
	"github.com/thought-machine/please/src/fs"
)

var log = logging.Log

//go:embed remote.so.gz
var remotePlugin []byte

//go:embed remote.so.gz.sha256
var remotePluginHash string

//go:embed prometheus.so.gz
var promPlugin []byte

//go:embed prometheus.so.gz.sha256
var promPluginHash string

//go:embed verify.so.gz
var verifyPlugin []byte

//go:embed verify.so.gz.sha256
var verifyPluginHash string

// LoadSymbol loads a symbol for a known plugin.
func LoadSymbol[T any](plugin, symbol string) (T, error) {
	sym, err := loadPlugin(plugin, symbol)
	if err != nil {
		var t T
		return t, err
	}
	return sym.(T), nil
}

// MustLoadSymbol is like LoadSymbol but dies on errors
func MustLoadSymbol[T any](plugin, symbol string) T {
	t, err := LoadSymbol[T](plugin, symbol)
	if err != nil {
		log.Fatalf("Failed to initialise %s plugin: %s", plugin, err)
	}
	return t
}

func loadPlugin(plugin, symbol string) (plugin.Symbol, error) {
	switch plugin {
	case "prometheus":
		return loadSymbol(promPlugin, promPluginHash, plugin, symbol)
	case "remote":
		return loadSymbol(remotePlugin, remotePluginHash, plugin, symbol)
	case "verify":
		return loadSymbol(verifyPlugin, verifyPluginHash, plugin, symbol)
	}
	panic("unknown plugin " + plugin)
}

func loadSymbol(data []byte, hash, name, sym string) (plugin.Symbol, error) {
	log.Debug("Loading plugin %s...", name)
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "please")
	basename := name + "_" + hash[:12] + ".so"
	filename := filepath.Join(dir, basename)
	if !fs.PathExists(filename) {
		log.Debug("Plugin %s doesn't exist, extracting...", name)
		if err := os.MkdirAll(dir, fs.DirPermissions); err != nil {
			return nil, err
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("Plugins not enabled in this build of Please")
		}
		gzr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		tmp, err := os.CreateTemp(dir, basename+".tmp_*")
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(tmp, gzr); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return nil, err
		}
		if err := tmp.Close(); err != nil {
			return nil, err
		}
		if err := os.Rename(tmp.Name(), filename); err != nil {
			return nil, err
		}
		log.Debug("Extracted %s", name)
	}
	p, err := plugin.Open(filename)
	if err != nil {
		return nil, err
	}
	return p.Lookup(sym)
}
