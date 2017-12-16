package gc

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"core"
)

func TestRewriteFile(t *testing.T) {
	state := core.NewBuildState(0, nil, 4, core.DefaultConfiguration())
	state.Config.Parse.PyLib = "src"
	// Copy file to avoid any issues with links etc.
	wd, _ := os.Getwd()
	err := core.CopyFile("src/gc/test_data/before.build", path.Join(wd, "test.build"), 0644)
	assert.NoError(t, err)
	assert.NoError(t, RewriteFile(state, "test.build", []string{"prometheus", "cover"}))
	rewritten, err := ioutil.ReadFile("test.build")
	assert.NoError(t, err)
	after, err := ioutil.ReadFile("src/gc/test_data/after.build")
	assert.NoError(t, err)
	assert.EqualValues(t, string(after), string(rewritten))
}
