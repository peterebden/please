package vfs

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"core"
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

func TestLink(t *testing.T) {
	fs := Must("vfs.TestLink").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1", "src/vfs/test_data/dir1")
	s := fs.Link("dir1/test.txt", "dir1/test2.txt", context)
	assert.Equal(t, fuse.OK, s)
	assert.True(t, core.IsSameFile(path.Join(fs.Root, "dir1/test.txt"), path.Join(fs.Root, "dir1/test2.txt")))
}

func TestMkdir(t *testing.T) {
	fs := Must("vfs.TestMkdir").(*filesystem)
	defer fs.Stop()
	s := fs.Mkdir("dir2", uint32(core.DirPermissions), context)
	assert.Equal(t, fuse.OK, s)
	info, err := os.Stat(path.Join(fs.Root, "dir2"))
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestRename(t *testing.T) {
	fs := Must("vfs.TestRename").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1", "src/vfs/test_data/dir1")
	s := fs.Rename("dir1", "dir2", context)
	assert.Equal(t, fuse.OK, s)
	info, err := os.Stat(path.Join(fs.Root, "dir2"))
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
	_, err = os.Stat(path.Join(fs.Root, "dir1"))
	assert.True(t, os.IsNotExist(err))
}

func TestRmdirRO(t *testing.T) {
	fs := Must("vfs.TestRmdirRO").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1", "src/vfs/test_data/dir1")
	s := fs.Rmdir("dir1", context)
	assert.Equal(t, fuse.EROFS, s)
}

func TestRmdirRW(t *testing.T) {
	fs := Must("vfs.TestRmdirRW").(*filesystem)
	defer fs.Stop()
	dirname := path.Join(fs.Root, "test")
	err := os.Mkdir(dirname, core.DirPermissions)
	assert.NoError(t, err)
	s := fs.Rmdir("test", context)
	assert.Equal(t, fuse.OK, s)
}

func TestUnlinkRO(t *testing.T) {
	fs := Must("vfs.TestUnlinkRO").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1/test.txt", "src/vfs/test_data/dir1/test.txt")
	s := fs.Unlink("dir1/test.txt", context)
	assert.Equal(t, fuse.EROFS, s)
}

func TestUnlinkRW(t *testing.T) {
	fs := Must("vfs.TestUnlinkRW").(*filesystem)
	defer fs.Stop()
	filename := addFile(t, fs, "test.txt")
	s := fs.Unlink("test.txt", context)
	assert.Equal(t, fuse.OK, s)
	_, err := os.Stat(filename)
	assert.True(t, os.IsNotExist(err))
}

func TestOpenRORDONLY(t *testing.T) {
	fs := Must("vfs.TestOpenRORDONLY").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1/test.txt", "src/vfs/test_data/dir1/test.txt")
	f, s := fs.Open("dir1/test.txt", uint32(os.O_RDONLY), context)
	assert.Equal(t, fuse.OK, s)
	dest := make([]byte, 7)
	r, s := f.Read(dest, 4)
	assert.Equal(t, fuse.OK, s)
	b, s := r.Bytes(dest)
	assert.Equal(t, fuse.OK, s)
	assert.EqualValues(t, []byte("testing"), b)
	assert.EqualValues(t, []byte("testing"), dest)
}

func TestOpenROWRONLY(t *testing.T) {
	fs := Must("vfs.TestOpenROWRONLY").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1/test.txt", "src/vfs/test_data/dir1/test.txt")
	_, s := fs.Open("dir1/test.txt", uint32(os.O_WRONLY), context)
	assert.Equal(t, fuse.EROFS, s)
}

func TestOpenRWRDONLY(t *testing.T) {
	fs := Must("vfs.TestOpenRWRDONLY").(*filesystem)
	defer fs.Stop()
	addFile(t, fs, "test.txt")
	f, s := fs.Open("test.txt", uint32(os.O_RDONLY), context)
	assert.Equal(t, fuse.OK, s)
	dest := make([]byte, 4)
	r, s := f.Read(dest, 0)
	assert.Equal(t, fuse.OK, s)
	b, s := r.Bytes(dest)
	assert.Equal(t, fuse.OK, s)
	assert.EqualValues(t, []byte("test"), b)
	assert.EqualValues(t, []byte("test"), dest)
}

func TestCreate(t *testing.T) {
	fs := Must("vfs.TestCreate").(*filesystem)
	defer fs.Stop()
	f, s := fs.Create("test.txt", uint32(os.O_WRONLY), 0644, context)
	assert.Equal(t, fuse.OK, s)
	n, s := f.Write([]byte("test"), 0)
	assert.EqualValues(t, 4, n)
	assert.Equal(t, fuse.OK, s)
}

func TestOpenDirRO(t *testing.T) {
	fs := Must("vfs.TestOpenDirRO").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1/test.txt", "src/vfs/test_data/dir1/test.txt")
	entries, s := fs.OpenDir("dir1", context)
	assert.Equal(t, fuse.OK, s)
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, "test.txt", entries[0].Name)
}

func TestOpenDirRW(t *testing.T) {
	fs := Must("vfs.TestOpenDirRW").(*filesystem)
	defer fs.Stop()
	dirname := path.Join(fs.Root, "test")
	err := os.Mkdir(dirname, core.DirPermissions)
	assert.NoError(t, err)
	addFile(t, fs, "test/test.txt")
	entries, s := fs.OpenDir("test", context)
	assert.Equal(t, fuse.OK, s)
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, "test.txt", entries[0].Name)
}

func TestOpenDirBoth(t *testing.T) {
	t.Skip("Doesn't work correctly yet")
	fs := Must("vfs.TestOpenDirBoth").(*filesystem)
	defer fs.Stop()
	fs.AddFile("test/test.txt", "src/vfs/test_data/dir1/test.txt")
	dirname := path.Join(fs.Root, "test")
	err := os.Mkdir(dirname, core.DirPermissions)
	assert.NoError(t, err)
	addFile(t, fs, "test/test2.txt")
	entries, s := fs.OpenDir("test", context)
	assert.Equal(t, fuse.OK, s)
	assert.Equal(t, 2, len(entries))
}

func TestSymlink(t *testing.T) {
	fs := Must("vfs.TestSymlink").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1/test.txt", "src/vfs/test_data/dir1/test.txt")
	s := fs.Symlink("dir1/test.txt", "dir1/test2.txt", context)
	assert.Equal(t, fuse.OK, s)
}

func TestReadlink(t *testing.T) {
	fs := Must("vfs.TestReadlink").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1/test.txt", "src/vfs/test_data/dir1/test.txt")
	fs.AddFile("dirlink", "src/vfs/test_data/dirlink")
	link, s := fs.Readlink("dirlink", context)
	assert.Equal(t, fuse.OK, s)
	assert.Equal(t, "dir1", link)
}

func TestSymlinkAndReadlink(t *testing.T) {
	fs := Must("vfs.TestSymlinkAndReadlink").(*filesystem)
	defer fs.Stop()
	fs.AddFile("dir1/test.txt", "src/vfs/test_data/dir1/test.txt")
	s := fs.Symlink("dir1/test.txt", "dir1/test2.txt", context)
	link, s := fs.Readlink("dir1/test2.txt", context)
	assert.Equal(t, fuse.OK, s)
	// TODO(peterebden): not sure this is correct...
	assert.Equal(t, "src/vfs/test_data/dir1/test.txt", link)
}

func TestExtract(t *testing.T) {
	fs := Must("vfs.TestExtract").(*filesystem)
	defer fs.Stop()
	addFile(t, fs, "test.txt")
	assert.False(t, exists("extract.txt"))
	err := fs.Extract("test.txt", "extract.txt")
	assert.NoError(t, err)
	assert.True(t, exists("extract.txt"))
}

func TestExtractSuggestions(t *testing.T) {
	fs := Must("vfs.TestExtractSuggestions").(*filesystem)
	defer fs.Stop()
	addFile(t, fs, "test.txt")
	addFile(t, fs, "wibble")
	err := fs.Extract("test2.txt", "suggest.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Maybe you meant test.txt")
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
