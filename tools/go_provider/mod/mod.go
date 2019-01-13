// Package mod handles reading go.mod files and generating individual
// libraries from it.
package mod

import (
	"sync"

	"cmd/go/internal/modfile"
)

// A Module represents a single Go module.
type Module struct {
	Path, Version string
}

var replacements = map[Module]Module{}
var mutex sync.Mutex

// Parse parses a go.mod file into a series of modules.
func Parse(filename string) ([]Module, error) {
	f, err := modfile.ParseLax(filename, nil, nil)
	if err != nil {
		return nil, err
	}
	mutex.Lock()
	defer mutex.Unlock()
	for _, repl := range f.Replace {
		replacements[Module(repl.Old)] = Module(repl.New)
	}
	for _, excl := range f.Exclude {
		replacements[Module(excl.Mod)] = Module{}
	}
	ret := make([]Module, len(f.Require))
	for i, req := range f.Require {
		mod := Module(req.Mod)
		if repl, present := replacements[mod]; present {
			if repl.Path != "" {
				ret[i] = repl
			}
		} else {
			ret[i] = mod
		}
	}
	return nil, nil
}
