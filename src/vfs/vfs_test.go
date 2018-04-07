package vfs

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var context = &fuse.Context{}

func TestCleanup(t *testing.T) {
	fs := Must("vfs.TestCleanup").(*filesystem)
	addFile(t, fs, "test.txt")
	fs.Stop()
	assert.False(t, exists(fs.Temp))
}

func TestGetAttrRO(t *testing.T) {
	fs := Must("vfs.TestGetAttrRO").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1", "src/vfs/test_data/dir1")
	attr, s := fs.GetAttr("dir1/test.txt", context)
	assert.Equal(t, fuse.OK, s)
	assert.True(t, attr.Size > 10)
}

func TestGetAttrRW(t *testing.T) {
	fs := Must("vfs.TestGetAttrRW").(*filesystem)
	defer fs.Stop()
	addFile(t, fs, "test.txt")
	attr, s := fs.GetAttr("test.txt", context)
	assert.Equal(t, fuse.OK, s)
	assert.True(t, attr.Size >= 4)
}

func TestChmodRO(t *testing.T) {
	fs := Must("vfs.TestChmodRO").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1", "src/vfs/test_data/dir1")
	s := fs.Chmod("dir1/test.txt", 0644, context)
	assert.Equal(t, fuse.EROFS, s)
}

func TestChmodRW(t *testing.T) {
	fs := Must("vfs.TestChmodRW").(*filesystem)
	defer fs.Stop()
	filename := addFile(t, fs, "test.txt")
	s := fs.Chmod("test.txt", 0755, context)
	assert.Equal(t, fuse.OK, s)
	info, err := os.Stat(filename)
	assert.NoError(t, err)
	assert.EqualValues(t, 0755, info.Mode())
}

func TestChownRO(t *testing.T) {
	fs := Must("vfs.TestChownRO").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1", "src/vfs/test_data/dir1")
	s := fs.Chown("dir1/test.txt", 1, 1, context)
	assert.Equal(t, fuse.EROFS, s)
}

// We don't do ChownRW because the underlying OS will likely prohibit the operation.

func TestTruncateRO(t *testing.T) {
	fs := Must("vfs.TestTruncateRO").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1", "src/vfs/test_data/dir1")
	s := fs.Truncate("dir1/test.txt", 1, context)
	assert.Equal(t, fuse.EROFS, s)
}

func TestTruncateRW(t *testing.T) {
	fs := Must("vfs.TestTruncateRW").(*filesystem)
	defer fs.Stop()
	filename := addFile(t, fs, "test.txt")
	s := fs.Truncate("test.txt", 1, context)
	assert.Equal(t, fuse.OK, s)
	info, err := os.Stat(filename)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, info.Size())
}

// Test helper to add an arbitrary file to the filesystem.
func addFile(t *testing.T, fs *filesystem, name string) string {
	name = path.Join(fs.Root, name)
	err := ioutil.WriteFile(name, []byte("test"), 0644)
	require.NoError(t, err)
	return name
}

// Returns true if the given path exists.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
