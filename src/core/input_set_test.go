package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	input1 = FileLabel{File: "test1.go", Package: "src/core"}
	input2 = FileLabel{File: "test2.go", Package: "src/core"}
	input3 = BuildLabel{PackageName: "src/core", Name: "core"}
)

func TestAddNamed(t *testing.T) {
	var s InputSet
	s.AddNamed("test", input1)
	s.AddNamed("test", input2)
	s.AddNamed("dep", input3)
	assert.Equal(t, 3, s.Count())
	assert.Equal(t, []BuildInput{input1, input2}, s.Named("test"))
	assert.Equal(t, []BuildInput{input1, input2, input3}, s.All())
	assert.True(t, s.IsNamed())
	assert.Equal(t, []string{"test", "dep"}, s.Names())
}

func TestAdd(t *testing.T) {
	var s InputSet
	s.Add(input1)
	s.Add(input2)
	s.Add(input3)
	assert.Equal(t, 3, s.Count())
	assert.Equal(t, []BuildInput(nil), s.Named("test"))
	assert.Equal(t, []BuildInput{input1, input2, input3}, s.All())
	assert.False(t, s.IsNamed())
	assert.Equal(t, 0, len(s.Names()))
}

func TestAddPanics(t *testing.T) {
	var s InputSet
	s.AddNamed("test", input1)
	s.AddNamed("test", input2)
	assert.Panics(t, func() { s.Add(input3) })
	s.AddNamed("dep", input3)
	assert.Panics(t, func() { s.Add(input3) })
}

func TestSet(t *testing.T) {
	var s InputSet
	s.Set([]BuildInput{input1, input3, input2})
	assert.Equal(t, []BuildInput{input1, input3, input2}, s.All())
}

func TestAddAllNamed(t *testing.T) {
	var s InputSet
	s.AddAllNamed("test", []BuildInput{input1, input2})
	s.AddAllNamed("dep", []BuildInput{input3})
	assert.Equal(t, []BuildInput{input1, input2, input3}, s.All())
}
