package cache

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"core"
)

var hash = []byte("12345678901234567890")
var b64Hash = base64.URLEncoding.EncodeToString(hash)

func writeFile(path string, size int) {
	contents := bytes.Repeat([]byte{'p', 'l', 'z'}, size) // so this is three times the size...
	if err := ioutil.WriteFile(path, contents, 0644); err != nil {
		panic(err)
	}
}

func inCache(target *core.BuildTarget) bool {
	dest := path.Join(".plz-cache", target.Label.PackageName, target.Label.Name, b64Hash, target.Outputs()[0])
	log.Debug("Checking for %s", dest)
	return core.PathExists(dest)
}

func TestStoreAndRetrieve(t *testing.T) {
	cache := makeCache()
	target := makeTarget("//test1:target1", 20)
	cache.Store(target, hash)
	// Should now exist in cache at this path
	assert.True(t, inCache(target))
	assert.True(t, cache.Retrieve(target, hash))
	// Should be able to store it again without problems
	cache.Store(target, hash)
	assert.True(t, inCache(target))
	assert.True(t, cache.Retrieve(target, hash))
}

func TestClean(t *testing.T) {
	cache := makeCache()
	target1 := makeTarget("//test1:target1", 2000)
	cache.Store(target1, hash)
	assert.True(t, inCache(target1))
	target2 := makeTarget("//test1:target2", 2000)
	cache.Store(target2, hash)
	assert.True(t, inCache(target2))
	// Doesn't clean anything this time because the high water mark is sufficiently high
	totalSize := cache.clean(20000, 1000)
	assert.EqualValues(t, 12000, totalSize)
	assert.True(t, inCache(target1))
	assert.True(t, inCache(target2))
}

func makeCache() *dirCache {
	config := core.DefaultConfiguration()
	config.Cache.DirClean = false // We will do this explicitly
	return newDirCache(config)
}

func makeTarget(label string, size int) *core.BuildTarget {
	target := core.NewBuildTarget(core.ParseBuildLabel(label, ""))
	target.AddOutput("test.go")
	os.MkdirAll(path.Join("plz-out/gen", target.Label.PackageName), core.DirPermissions)
	writeFile(path.Join("plz-out/gen", target.Label.PackageName, "test.go"), size)
	return target
}
