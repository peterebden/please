package vfs

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var context = &fuse.Context{}

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

// Test helper to add an arbitrary file to the filesystem.
func addFile(t *testing.T, fs *filesystem, name string) {
	err := ioutil.WriteFile(path.Join(fs.Root, name), []byte("test"), 0644)
	require.NoError(t, err)
}
