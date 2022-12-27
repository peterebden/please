package core

import (
	"sync"

	"github.com/thought-machine/please/src/fs"
)

// A TargetHasher handles hash calculation for a target.
type TargetHasher struct {
	hasher *fs.PathHasher
	hashes map[*BuildTarget][]byte
	mutex  sync.RWMutex
}

// OutputHash calculates the standard output hash of a build target.
func (h *TargetHasher) OutputHash(target *BuildTarget) ([]byte, error) {
	h.mutex.RLock()
	hash, present := h.hashes[target]
	h.mutex.RUnlock()
	if present {
		return hash, nil
	}
	hash, err := h.outputHash(target, false)
	if err != nil {
		return nil, err
	}
	h.SetHash(target, hash)
	return hash, nil
}

// ForceOutputHash is like OutputHash but always forces a recalculation (i.e. it never memoises).
func (h *TargetHasher) ForceOutputHash(target *BuildTarget) ([]byte, error) {
	hash, err := h.outputHash(target, true)
	if err != nil {
		return nil, err
	}
	h.SetHash(target, hash)
	return hash, nil
}

// SetHash sets a hash for a build target.
func (h *TargetHasher) SetHash(target *BuildTarget, hash []byte) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.hashes[target] = hash
}

// outputHash calculates the output hash for a target, choosing an appropriate strategy.
func (h *TargetHasher) outputHash(target *BuildTarget, force bool) ([]byte, error) {
	return OutputHashOfType(target, target.FullOutputs(), force, h.hasher)
}

// OutputHashOfType is a more general form of OutputHash that allows different hashing strategies.
func OutputHashOfType(target *BuildTarget, outputs []string, force bool, hasher *fs.PathHasher) ([]byte, error) {
	timestamp := target.HashLastModified()
	if len(outputs) == 1 {
		// Single output, just hash that directly.
		return hasher.Hash(outputs[0], force, !target.IsFilegroup, timestamp)
	}
	h := hasher.NewHash()
	for _, filename := range outputs {
		// NB. Always force a recalculation of the output hashes here. Memoisation is not
		//     useful because by definition we are rebuilding a target, and can actively hurt
		//     in cases where we compare the retrieved cache artifacts with what was there before.
		h2, err := hasher.Hash(filename, force, !target.IsFilegroup, timestamp)
		if err != nil {
			return nil, err
		}
		h.Write(h2)
		// Record the name of the file too, but not if the rule has hash verification
		// (because this will change the hashes, and the cases it fixes are relatively rare
		// and generally involve things like hash_filegroup that doesn't have hashes set).
		// TODO(pebers): Find some more elegant way of unifying this behaviour.
		if len(target.Hashes) == 0 {
			h.Write([]byte(filename))
		}
	}
	return h.Sum(nil), nil
}
