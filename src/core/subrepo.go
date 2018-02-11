package core

import "strings"

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

// MakeRelative makes a build label that is within this subrepo relative to it (i.e. strips the leading name part).
// The caller should already know that it is within this repo, otherwise this will panic.
func (s *Subrepo) MakeRelative(label BuildLabel) BuildLabel {
	if !strings.HasPrefix(label.PackageName, s.Name) {
		panic("cannot make label relative, it is not within this subrepo")
	}
	return BuildLabel{strings.TrimPrefix(label.PackageName[len(s.Name):], "/"), label.Name}
}
