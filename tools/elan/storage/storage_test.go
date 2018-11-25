package storage

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"sync"
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

	// This is a little clunky; this needs to be big enough for io.Copy to make > 1
	// call for the test to work properly, but obviously we don't want to link too
	// directly to what that actually is. We just "guess" that this is enough.
	content := bytes.Repeat([]byte{1}, 1024*1024)
	tr := &testReader{r: bytes.NewReader(content)}
	tr.Mutex.Lock()

	const hash = 12345
	const name = "test_file"
	go func() {
		err := s.Save(hash, name, tr)
		assert.NoError(t, err)
		log.Notice("Saved file %s", name)
	}()

	// This makes sure it's been called once, so we won't get an ErrNotExist next.
	tr.Mutex.Lock()

	// Now once we read it back, we should get the whole thing.
	r, err := s.Load(hash, name)
	assert.NoError(t, err)
	b, err := ioutil.ReadAll(r)
	assert.NoError(t, err)
	assert.EqualValues(t, len(content), len(b))
}

type testReader struct {
	Mutex sync.Mutex
	count int
	r     io.Reader
}

func (t *testReader) Read(b []byte) (int, error) {
	if t.count == 0 {
		defer t.Mutex.Unlock()
	} else if t.count == 1 {
		// Sleep a bit to give the reader a chance to get ahead
		time.Sleep(100 * time.Millisecond)
	}
	t.count++
	return t.r.Read(b)
}

func (t *testReader) Close() error {
	return nil
}
