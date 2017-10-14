package zip

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"third_party/go/zip"
)

var expectedModTime = time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC)

func TestAddZipFile(t *testing.T) {
	// Have to write an actual file for zip.OpenReader to use later.
	f := NewFile("add_zip_file_test.zip", false)
	err := f.AddZipFile("tools/jarcat/zip/test_data/test.zip")
	require.NoError(t, err)
	f.Close()
	assertExpected(t, "add_zip_file_test.zip")
}

func TestAddFiles(t *testing.T) {
	f := NewFile("add_files_test.zip", false)
	f.Suffix = []string{"zip"}
	err := f.AddFiles("tools")
	require.NoError(t, err)
	f.Close()
	assertExpected(t, "add_files_test.zip")
}

func assertExpected(t *testing.T, filename string) {
	r, err := zip.OpenReader(filename)
	if err != nil {
		t.Fatalf("Failed to reopen zip file: %s", err)
	}
	defer r.Close()
	files := []struct{ Name, Prefix string }{
		{"build_step.go", "// Implementation of Step interface."},
		{"incrementality.go", "// Utilities to help with incremental builds."},
	}
	for i, f := range r.File {
		assert.Equal(t, f.Name, files[i].Name)
		assert.Equal(t, expectedModTime, f.ModTime())

		fr, err := f.Open()
		require.NoError(t, err)
		var buf bytes.Buffer
		_, err = io.Copy(&buf, fr)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(buf.String(), files[i].Prefix))
		fr.Close()
	}
}
