package format

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thought-machine/please/src/core"
)

func TestFormatInPlace(t *testing.T) {
	filename := "src/format/test_data/test_file.txt"
	state := core.NewDefaultBuildState().ForConfig("src/format/test_data/inplace.plzconfig")
	err := Reformat(state, filename)
	assert.NoError(t, err)
	b, err := ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, "correctly formatted\n", string(b))
}

func TestFormatNotInPlace(t *testing.T) {
	filename := "src/format/test_data/test_file_2.txt"
	state := core.NewDefaultBuildState().ForConfig("src/format/test_data/notinplace.plzconfig")
	err := Reformat(state, filename)
	assert.NoError(t, err)
	b, err := ioutil.ReadFile(filename)
	assert.NoError(t, err)
	assert.Equal(t, "correctly formatted\n", string(b))
}
