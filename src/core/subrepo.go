package core

// A Subrepo stores information about a registered subrepository, typically one
// that we have downloaded somehow to bring in third-party deps.
type Subrepo struct {
	// The name of the subrepo.
	Name string
	// The root directory to load it from.
	Root string
	// If this repo is output by a target, this is the target that creates it. Can be nil.
	Target *BuildTarget
}
