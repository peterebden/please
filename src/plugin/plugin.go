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
	"github.com/thought-machine/please/src/core"
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

// LoadPlugins loads the relevant plugins for the current config
func LoadPlugins(state *core.BuildState) error {
	if state.Config.Remote.URL != "" {
		sym, err := loadPlugin(remotePlugin, remotePluginHash, "remote", "New")
		if err != nil {
			return fmt.Errorf("Remote: %w", err)
		}
		f := sym.(func(state *core.BuildState) core.RemoteClient)
		state.RemoteClient = f(state)
	}
	if state.Config.Metrics.PrometheusGatewayURL != "" {
		sym, err := loadPlugin(promPlugin, promPluginHash, "prometheus", "Register")
		if err != nil {
			return fmt.Errorf("Prometheus: %w", err)
		}
		f := sym.(func())
		f()
	}
	return nil
}

// LoadSymbol loads a symbol for a known plugin.
func LoadSymbol[T any](plugin, symbol string) (T, error) {
	if plugin != "verify" {
		// You can't load the other two plugins this way.
		panic("unknown plugin " + plugin)
	}
	sym, err := loadPlugin(verifyPlugin, verifyPluginHash, "verify", "VerifySignature")
	if err != nil {
		var t T
		return t, err
	}
	return sym.(T), nil
}

func loadPlugin(data []byte, hash, name, sym string) (plugin.Symbol, error) {
	log.Debug("Loading plugin %s...", name)
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "please")
	basename := name + hash[:12] + ".so"
	filename := filepath.Join(dir)
	if !fs.PathExists(filename) {
		log.Debug("Plugin %s doesn't exist, extracting...", name)
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
