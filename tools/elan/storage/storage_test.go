package storage

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

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

	w, err := s.Save(hash, name)
	assert.NoError(t, err)
	n, err := io.Copy(w, bytes.NewReader(content))
	assert.NoError(t, err)
	assert.EqualValues(t, len(content), n)
	err = w.Close()
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

	r, err = s.Load(hash, name)
	assert.NoError(t, err)
	b, err = ioutil.ReadAll(r)
	assert.NoError(t, err)
	assert.EqualValues(t, content, b)
}

func TestConcurrentSaveAndLoad(t *testing.T) {
	s, err := Init("test3", 9999999)
	assert.NoError(t, err)
	defer s.Shutdown()

	const hash = 12345
	const name = "test_file"
	content := bytes.Repeat([]byte{1}, 1024*1024)

	// Open writer and write a bit so the file is stored
	w, err := s.Save(hash, name)
	assert.NoError(t, err)
	n, err := w.Write(content[:1024])
	assert.NoError(t, err)
	assert.EqualValues(t, 1024, n)

	// Now kick off concurrent writing
	go func() {
		// Sleep a little to let the reader (probably) get ahead, otherwise we surprisingly
		// often seem to complete before it does.
		time.Sleep(50 * time.Millisecond)
		n, err := io.Copy(w, bytes.NewReader(content[1024:]))
		assert.NoError(t, err)
		assert.EqualValues(t, len(content)-1024, n)
		err = w.Close()
		assert.NoError(t, err)
	}()

	// Now once we read it back, we should get the whole thing.
	r, err := s.Load(hash, name)
	assert.NoError(t, err)
	b, err := ioutil.ReadAll(r)
	assert.NoError(t, err)
	assert.EqualValues(t, len(content), len(b))
}
