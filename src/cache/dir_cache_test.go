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
	cache := makeCache("0", "10M")
	target := makeTarget("//test1:target1", 0)
	cache.Store(target, hash)
	// Should now exist in cache at this path
	assert.True(t, inCache(target))
}

func makeCache(lowWaterMark, highWaterMark string) *dirCache {
	config := core.DefaultConfiguration()
	config.Cache.DirCacheLowWaterMark.UnmarshalFlag(lowWaterMark)
	config.Cache.DirCacheHighWaterMark.UnmarshalFlag(highWaterMark)
	return newDirCache(config)
}

func makeTarget(label string, size int) *core.BuildTarget {
	target := core.NewBuildTarget(core.ParseBuildLabel(label, ""))
	target.AddOutput("test.go")
	os.MkdirAll(path.Join("plz-out/gen", target.Label.PackageName), core.DirPermissions)
	writeFile(path.Join("plz-out/gen", target.Label.PackageName, "test.go"), size)
	return target
}
