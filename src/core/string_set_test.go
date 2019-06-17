package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	input1 = "test1.go"
	input2 = "test2.go"
	input3 = "//src/core:core"
)

func TestAddNamed(t *testing.T) {
	var s StringSet
	s.AddNamed("test", input1)
	s.AddNamed("test", input2)
	s.AddNamed("dep", input3)
	assert.Equal(t, 3, s.Count())
	assert.Equal(t, []string{input1, input2}, s.Named("test"))
	assert.Equal(t, []string{input1, input2, input3}, s.All())
	assert.True(t, s.IsNamed())
	assert.Equal(t, []string{"test", "dep"}, s.Names())
}

func TestAdd(t *testing.T) {
	var s StringSet
	s.Add(input1)
	s.Add(input2)
	s.Add(input3)
	assert.Equal(t, 3, s.Count())
	assert.Equal(t, []string(nil), s.Named("test"))
	assert.Equal(t, []string{input1, input2, input3}, s.All())
	assert.False(t, s.IsNamed())
	assert.Equal(t, 0, len(s.Names()))
}

func TestAddPanics(t *testing.T) {
	var s StringSet
	s.AddNamed("test", input1)
	s.AddNamed("test", input2)
	assert.Panics(t, func() { s.Add(input3) })
	s.AddNamed("dep", input3)
	assert.Panics(t, func() { s.Add(input3) })
}

func TestSet(t *testing.T) {
	var s StringSet
	s.Set([]string{input1, input3, input2})
	assert.Equal(t, []string{input1, input3, input2}, s.All())
}

func TestAddAllNamed(t *testing.T) {
	var s StringSet
	s.AddAllNamed("test", []string{input1, input2})
	s.AddAllNamed("dep", []string{input3})
	assert.Equal(t, []string{input1, input2, input3}, s.All())
}
