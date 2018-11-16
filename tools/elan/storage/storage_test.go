package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadAndSaveConfig(t *testing.T) {
	s, err := Init(3, 4, "test", 9999999)
	assert.NoError(t, err)
	expectedTokens := []uint64{
		0,
		4611686018427387903,
		9223372036854775806,
		13835058055282163709,
	}
	assert.Equal(t, expectedTokens, s.Tokens())
	s.Shutdown()

	// Reinitialise with a different directory, tokens should be different
	s, err = Init(3, 7, "test2", 9999999)
	assert.NoError(t, err)
	assert.NotEqual(t, expectedTokens, s.Tokens())

	// Now loading the original directory with a different number of tokens should reload the originals
	s, err = Init(3, 7, "test", 9999999)
	assert.NoError(t, err)
	assert.Equal(t, expectedTokens, s.Tokens())
}
