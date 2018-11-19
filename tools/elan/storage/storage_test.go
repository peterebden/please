package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadAndSaveConfig(t *testing.T) {
	s, err := Init("test", 9999999)
	assert.NoError(t, err)
	c, err := s.LoadConfig()
	assert.NoError(t, err)
	assert.False(t, c.Initialised)
	c.Initialised = true
	err = s.SaveConfig(c)
	assert.NoError(t, err)
	c, err = s.LoadConfig()
	assert.NoError(t, err)
	assert.True(t, c.Initialised)
}
