// Package unzip implements unzipping for jarcat.
// We implement this to avoid needing a runtime dependency on unzip,
// which is not a profound package but not installed everywhere by default.
package unzip

import (
	"io"
	"os"
	"path"
	"strings"

	"third_party/go/zip"
)

// Extract extracts the contents of the given zipfile.
func Extract(in, out, file, prefix string) error {
	e := extractor{
		In:     in,
		Out:    out,
		File:   file,
		Prefix: prefix,
		dirs:   map[string]struct{}{},
	}
	return e.Extract()
}

// An extractor extracts a single zipfile.
type extractor struct {
	In     string
	Out    string
	File   string
	Prefix string
	dirs   map[string]struct{}
}

func (e *extractor) Extract() error {
	r, err := zip.OpenReader(e.In)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if e.File != "" && f.Name != e.File {
			continue
		}
		// This will mean that empty directories aren't created. We might need to fix that at some point.
		if f.Mode()&os.ModeDir == 0 {
			if err := e.extractFile(f); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *extractor) extractFile(f *zip.File) error {
	if e.Prefix != "" {
		if !strings.HasPrefix(f.Name, e.Prefix) {
			return nil
		}
		f.Name = strings.TrimLeft(strings.TrimPrefix(f.Name, e.Prefix), "/")
	}
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	out := path.Join(e.Out, f.Name)
	if e.File != "" {
		out = e.Out
	}
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
