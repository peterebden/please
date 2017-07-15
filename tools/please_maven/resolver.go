package maven

import (
	"sync"
)

// A Resolver resolves Maven artifacts into specific versions.
// Ultimately we should really be using a proper SAT solver here but we're
// not rushing it in favour of forcing manual disambiguation (and the majority
// of things going wrong are due to Maven madness anyway).
type Resolver struct {
	sync.Mutex
	// Contains all the poms we've fetched.
	// Note that these are keyed by a subset of the artifact struct, so we
	// can do version-independent lookups.
	poms map[unversioned][]*pomXml
	// Reference to a thing that fetches for us.
	fetch *Fetch
}

// NewResolver constructs and returns a new Resolver instance.
func NewResolver(f *Fetch) *Resolver {
	return &Resolver{
		poms:  map[unversioned][]*pomXml{},
		fetch: f,
	}
}

// Pom returns a pom for an artifact. The version doesn't have to be specified exactly.
// If one doesn't currently exist it will return nil.
func (r *Resolver) Pom(a *Artifact) *pomXml {
	r.Lock()
	defer r.Unlock()
	poms := r.poms[a.unversioned]
	log.Debug("Resolving %s:%s: found %d candidates", a.GroupId, a.ArtifactId, len(poms))
	for _, pom := range poms {
		// TODO(peterebden): temporary hack that mimics older behaviour. Remove.
		return pom
		if r.VersionMatches(pom.Version, a.Version) {
			log.Debug("Retrieving pom %s for %s", pom.Id(), a.Id())
			return pom
		}
	}
	return nil
}

// VersionMatches returns true if a concrete version matches a version spec.
// TODO(peterebden): Implement proper Maven version specifier support.
func (r *Resolver) VersionMatches(concrete, spec string) bool {
	if spec == "" {
		return true // No version specified, must match.
	}
	return concrete == spec
}

// Store stores a retrieved pom in the resolver.
func (r *Resolver) Store(pom *pomXml) {
	log.Debug("Storing pom %s", pom.Id())
	r.Lock()
	defer r.Unlock()
	r.poms[pom.unversioned] = append(r.poms[pom.unversioned], pom)
}
