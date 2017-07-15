package maven

import (
	"sync"
	"sync/atomic"

	"github.com/Workiva/go-datastructures/queue"
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
	// Task queue that prioritises upcoming tasks.
	tasks *queue.PriorityQueue
	// Count of live tasks.
	liveTasks int64
}

// NewResolver constructs and returns a new Resolver instance.
func NewResolver(f *Fetch) *Resolver {
	return &Resolver{
		poms:  map[unversioned][]*pomXml{},
		fetch: f,
		tasks: queue.NewPriorityQueue(100, false),
	}
}

// Run runs the given number of worker threads until everything is resolved.
func (r *Resolver) Run(concurrency int) {
	// We use this channel as a slightly overblown semaphore; when any one
	// of the goroutines finishes, we're done. At least one will return but
	// not necessarily more than that.
	ch := make(chan bool, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			r.process()
			ch <- true
		}()
	}
	<-ch
}

// Pom returns a pom for an artifact. The version doesn't have to be specified exactly.
// If one doesn't currently exist it will return nil.
func (r *Resolver) Pom(a *Artifact) *pomXml {
	r.Lock()
	defer r.Unlock()
	return r.pom(a)
}

// CreatePom returns a pom for an artifact. If a suitable match doesn't exist, a new one
// will be created. The second return value is true if a new one was created.
func (r *Resolver) CreatePom(a *Artifact) (*pomXml, bool) {
	r.Lock()
	defer r.Unlock()
	if pom := r.pom(a); pom != nil {
		return pom, false
	}
	// Override an empty version with a suggestion if we're going to create it.
	if a.Version == "" {
		a.SetVersion(a.SoftVersion)
	}
	pom := &pomXml{Artifact: *a}
	r.poms[pom.unversioned] = append(r.poms[pom.unversioned], pom)
	return pom, true
}

func (r *Resolver) pom(a *Artifact) *pomXml {
	poms := r.poms[a.unversioned]
	log.Debug("Resolving %s:%s: found %d candidates", a.GroupId, a.ArtifactId, len(poms))
	for _, pom := range poms {
		if a.Version == "" || pom.ParsedVersion.Matches(&a.ParsedVersion) {
			log.Debug("Retrieving pom %s for %s", pom.Artifact, a)
			return pom
		}
	}
	return nil
}

// Submit adds this dependency to the queue for future resolution.
func (r *Resolver) Submit(dep *pomDependency) {
	atomic.AddInt64(&r.liveTasks, 1)
	r.tasks.Put(dep)
}

// process continually reads tasks from the queue and resolves them.
func (r *Resolver) process() {
	for {
		t, err := r.tasks.Get(1)
		if err != nil {
			log.Fatalf("%s", err)
		}
		dep := t[0].(*pomDependency)
		log.Debug("beginning resolution of %s", dep.Artifact)
		dep.Resolve(r.fetch)
		count := atomic.AddInt64(&r.liveTasks, -1)
		log.Debug("processed %s, %d tasks remaining", dep.Artifact, count)
		if count <= 0 {
			log.Debug("all tasks done, stopping")
			break
		}
	}
}
