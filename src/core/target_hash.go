package core

import (
	"hash"
	"sync"

	"github.com/thought-machine/please/src/fs"
)

// A TargetHasher handles hash calculation for a target.
type TargetHasher struct {
	state  *BuildState
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
	outs := target.FullOutputs()
	if len(outs) == 0 {
		// Special case for a single output because it's easier, but also allows for the file's
		// shasum to be the hash plz uses.
		return h.state.PathHasher.Hash(outs[0], force, !target.IsFilegroup)
	}
	sum := h.state.PathHasher.NewHash()
	for _, out := range outs {
		h2, err := h.state.PathHasher.Hash(out, force, !target.IsFilegroup)
		if err != nil {
			return nil, err
		}
		sum.Write(h2)
		// Record the name of the file too, but not if the rule has hash verification
		// (because this will change the hashes, and the cases it fixes are relatively rare
		// and generally involve things like hash_filegroup that doesn't have hashes set).
		// TODO(peterebden): Find some more elegant way of unifying this behaviour.
		if len(target.Hashes) == 0 {
			sum.Write([]byte(out))
		}
	}
	return sum.Sum(nil), nil
}

// OutputHashOfType is a more general form of OutputHash that allows different hashing strategies.
func OutputHashOfType(target *BuildTarget, outputs []string, hasher *fs.PathHasher, combine func() hash.Hash) ([]byte, error) {
	if combine == nil {
		// Must be a single output, just hash that directly.
		return hasher.Hash(outputs[0], true, target.IsFilegroup)
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
