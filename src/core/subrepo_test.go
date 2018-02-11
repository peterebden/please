package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakeRelative(t *testing.T) {
	s := &Subrepo{Name: "repo"}
	l := s.MakeRelative(NewBuildLabel("repo/package", "name"))
	assert.Equal(t, NewBuildLabel("package", "name"), l)
}
