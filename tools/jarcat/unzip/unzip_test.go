package unzip

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var files = []string{
	"third_party/python/xmlrunner/",
	"third_party/python/xmlrunner/__init__.py",
	"third_party/python/xmlrunner/__main__.py",
	"third_party/python/xmlrunner/builder.py",
	"third_party/python/xmlrunner/extra/",
	"third_party/python/xmlrunner/extra/__init__.py",
	"third_party/python/xmlrunner/extra/djangotestrunner.py",
	"third_party/python/xmlrunner/result.py",
	"third_party/python/xmlrunner/runner.py",
	"third_party/python/xmlrunner/unittest.py",
	"third_party/python/xmlrunner/version.py",
}

func TestExtract(t *testing.T) {
	e := Extractor{
		In:  "tools/jarcat/unzip/test_data/xmlrunner.whl",
		Out: ".",
	}
	assert.NoError(t, e.Extract())
	for _, file := range files {
		_, err := os.Stat(file)
		assert.NoError(t, err)
	}
}
