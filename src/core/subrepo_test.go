package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakeRelative(t *testing.T) {
	s := &Subrepo{Name: "repo"}
	l := s.MakeRelative(NewBuildLabel("repo/package", "name"))
	assert.Equal(t, BuildLabel{PackageName: "package", Name: "name", Subrepo: "repo"}, l)
	assert.Panics(t, func() { s.MakeRelative(NewBuildLabel("other/package", "name")) })
}

func TestMakeRelativeName(t *testing.T) {
	s := &Subrepo{Name: "com_google_googletest"}
	assert.Equal(t, "googletest/include", s.MakeRelativeName("com_google_googletest/googletest/include"))
}

func TestDir(t *testing.T) {
	s := &Subrepo{Name: "repo", Root: "plz-out/gen/repo"}
	assert.Equal(t, "plz-out/gen/repo/package", s.Dir("repo/package"))
	assert.Panics(t, func() { s.Dir("other/package") })
}
