// Package pex implements construction of .pex files in Go.
// For performance reasons we've ultimately abandoned doing this in Python;
// we were ultimately not using pex for much at construction time and
// we already have most of what we need in Go via jarcat.
package pex

import (
	"bytes"
	"os"
	"path"
	"strings"
	"zip"

	"tools/jarcat"
)

// A PexWriter implements writing a .pex file in various steps.
type PexWriter struct {
	shebang        string
	realEntryPoint string
	testSrcs       []string
	zipSafe        bool
}

// NewPexWriter constructs a new PexWriter.
func NewPexWriter(entryPoint, interpreter, codeHash string, zipSafe bool) *PexWriter {
	pw := &PexWriter{
		realEntryPoint: toPythonPath(entryPoint),
		zipSafe:        zipSafe,
	}
	pw.SetShebang(interpreter + " -S")
	return pw
}

// SetShebang sets the leading shebang that will be written to the file.
func (pw *PexWriter) SetShebang(shebang string) {
	if !path.IsAbs(shebang) {
		shebang = "/usr/bin/env " + shebang
	}
	if !strings.HasPrefix(shebang, "#") {
		shebang = "#!" + shebang
	}
	pw.shebang = shebang + "\n"
}

// SetTest sets this PexWriter to write tests using the given sources.
// This overrides the entry point given earlier.
func (pw *PexWriter) SetTest(srcs []string) {
	pw.testSrcs = make([]string, len(srcs))
	for i, src := range srcs {
		pw.testSrcs[i] = toPythonPath(src)
	}
}

// Write writes the pex to the given output file.
func (pw *PexWriter) Write(out, moduleDir string) error {
	// TODO(pebers): move this to jarcat package.
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()

	// Write preamble (i.e. the shebang that makes it executable)
	if err := w.WritePreamble([]byte(pw.shebang)); err != nil {
		return err
	}

	// Write required pex stuff. Note that this executable is also a zipfile and we can
	// jarcat it directly in (nifty, huh?).
	if err := jarcat.AddZipFile(w, os.Args[0], pw.zipIncludes(), nil, "", false, nil); err != nil {
		return err
	}
	// Always write pex_main.py, with some templating.
	b := MustAsset("pex_main.py")
	b = bytes.Replace(b, []byte("__MODULE_DIR__"), []byte(moduleDir), 1)
	b = bytes.Replace(b, []byte("__ENTRY_POINT__"), []byte(pw.realEntryPoint), 1)
	b = bytes.Replace(b, []byte("__ZIP_SAFE__"), []byte(pythonBool(pw.zipSafe)), 1)
	if len(pw.testSrcs) == 0 {
		// Not a test, pex_main becomes the entry point.
		return jarcat.WriteFile(w, "__main__.py", b)
	}
	// If we're writing a test, we'll need test_main.py too. It becomes our entry point.
	// We still use pex_main.py though.
	if err := jarcat.WriteFile(w, "pex_main.py", b); err != nil {
		return err
	}
	b = MustAsset("test_main.py")
	b = bytes.Replace(b, []byte("__TEST_NAMES__"), []byte(strings.Join(pw.testSrcs, ",")), 1)
	return jarcat.WriteFile(w, "__main__.py", b)
}

// zipIncludes returns the list of paths we'll include from our own zip file.
func (pw *PexWriter) zipIncludes() []string {
	// If we're writing a test, we can write the whole bootstrap dir and move on with our lives.
	if len(pw.testSrcs) != 0 {
		return []string{".bootstrap"}
	}
	// Always extract the following.
	// Note that we have a be a bit careful that these are a complete set of required paths.
	return []string{
		".bootstrap/pkg_resources",
		".bootstrap/__init__.py",
	}
}

// pythonBool returns a Python bool representation of a Go bool.
func pythonBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// toPythonPath converts a normal path to a Python import path.
func toPythonPath(p string) string {
	ext := path.Ext(p)
	return strings.Replace(p[:len(p)-len(ext)], "/", ".", -1)
}
