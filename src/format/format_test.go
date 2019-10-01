package format

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestFormatBuildFiles(t *testing.T) {
	const num_tests = 1
	for i := 1; i <= num_tests; i++ {
		t.Run(fmt.Sprintf("test%d", i), func(t *testing.T) {
			expected, err := ioutil.ReadFile(fmt.Sprintf("src/format/test_data/out%d.build", i))
			require.NoError(t, err)
			err = ReformatBuild(fmt.Sprintf("src/format/test_data/in%d.build", i))
			assert.NoError(t, err)
			actual, err := ioutil.ReadFile(fmt.Sprintf("src/format/test_data/in%d.build", i))
			require.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}
}
