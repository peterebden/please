// Package unzip implements unzipping for jarcat.
// We implement this to avoid needing a runtime dependency on unzip,
// which is not a profound package but not installed everywhere by default.
package unzip

import (
	"io"
	"os"
	"path"

	"gopkg.in/op/go-logging.v1"

	"third_party/go/zip"
)

var log = logging.MustGetLogger("unzip")

// An Extractor extracts a single zipfile.
type Extractor struct {
	In   string
	Out  string
	dirs map[string]struct{}
}

// Extract extracts the contents of the given zipfile.
func (e *Extractor) Extract() error {
	e.dirs = map[string]struct{}{}
	r, err := zip.OpenReader(e.In)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		// This will mean that empty directories aren't created. We might need to fix that at some point.
		if f.Mode()&os.ModeDir == 0 {
			log.Debug("extracting %s", f.Name)
			if err := e.extractFile(f); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Extractor) extractFile(f *zip.File) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	out := path.Join(e.Out, f.Name)
	dir := path.Dir(out)
	if _, present := e.dirs[dir]; !present {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		e.dirs[dir] = struct{}{}
	}
	o, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE, f.Mode())
	if err != nil {
		return err
	}
	defer o.Close()
	_, err = io.Copy(o, r)
	return err
}
