package core

import (
	"hash"
	"sync"

	"github.com/thought-machine/please/src/fs"
)

// A TargetHasher handles hash calculation for a target.
type TargetHasher struct {
	State  *BuildState
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
	hash, err := h.outputHash(target)
	if err != nil {
		return hash, err
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
func (h *TargetHasher) outputHash(target *BuildTarget) ([]byte, error) {
	outs := target.FullOutputs()

	// We must combine for sha1 for backwards compatibility
	// TODO(jpoole): remove this special case in v16
	mustCombine := h.State.Config.Build.HashFunction == "sha1" && !h.State.Config.FeatureFlags.SingleSHA1Hash
	combine := len(outs) != 1 || mustCombine

	if !combine && fs.FileExists(outs[0]) {
		return OutputHashOfType(target, outs, h.State.PathHasher, nil)
	}
	return OutputHashOfType(target, outs, h.State.PathHasher, h.State.PathHasher.NewHash)
}

// OutputHashOfType is a more general form of OutputHash that allows different hashing strategies.
func OutputHashOfType(target *BuildTarget, outputs []string, hasher *fs.PathHasher, combine func() hash.Hash) ([]byte, error) {
	if combine == nil {
		// Must be a single output, just hash that directly.
		return hasher.Hash(outputs[0], true, !target.IsFilegroup)
	}
	h := combine()
	for _, filename := range outputs {
		// NB. Always force a recalculation of the output hashes here. Memoisation is not
		//     useful because by definition we are rebuilding a target, and can actively hurt
		//     in cases where we compare the retrieved cache artifacts with what was there before.
		h2, err := hasher.Hash(filename, true, !target.IsFilegroup)
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
