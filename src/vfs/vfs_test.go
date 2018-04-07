package vfs

import (
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/stretchr/testify/assert"
)

var context = &fuse.Context{}

func TestGetAttrRO(t *testing.T) {
	fs := Must("TestGetAttrRO").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1", "src/vfs/test_data/dir1")
	attr, s := fs.GetAttr("dir1/test.txt", context)
	assert.Equal(t, fuse.OK, s)
	assert.True(t, attr.Size > 10)
}
