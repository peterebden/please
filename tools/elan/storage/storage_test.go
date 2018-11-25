package storage

import (
	"bytes"
	"io/ioutil"
	"os"
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

func TestFileStorage(t *testing.T) {
	s, err := Init("test2", 9999999)
	assert.NoError(t, err)

	const hash = 12345
	const name = "test_file"
	content := []byte("testtesttest")
	_, err = s.Load(hash, name)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	err = s.Save(hash, name, ioutil.NopCloser(bytes.NewReader(content)))
	assert.NoError(t, err)

	r, err := s.Load(hash, name)
	assert.NoError(t, err)
	b, err := ioutil.ReadAll(r)
	assert.NoError(t, err)
	assert.EqualValues(t, content, b)

	// Now restart it, and we should still be able to load the same file.
	s.Shutdown()
	s, err = Init("test2", 9999999)
	assert.NoError(t, err)
	log.Warning("%s", s.(*storage).files)

	r, err = s.Load(hash, name)
	assert.NoError(t, err)
	b, err = ioutil.ReadAll(r)
	assert.NoError(t, err)
	assert.EqualValues(t, content, b)
}
