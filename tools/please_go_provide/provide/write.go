// Package provide implements provisioning of BUILD files for go_module subrepos.
package provide

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

// Write writes BUILD files for all directories under the given path.
func Write(importPath, dir string, deps []string) error {
	provides := map[string]string{}
	binaries := map[string]string{}
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if !info.IsDir() || path == dir {
			return nil
		}
		return write(importPath, path[len(dir):], path, deps, provides, binaries)
	}); err != nil {
		return err
	}
	return write(importPath, "", dir, deps, provides, binaries)
}

// write writes a single BUILD file.
func write(rootImportPath, pkgName, dir string, deps []string, provides, binaries map[string]string) error {
	fs := token.NewFileSet()
	pkgs, err := parser.ParseDir(fs, dir, nonTestOnly, parser.ImportsOnly)
	if err != nil {
		return err
	}
	for _, pkg := range pkgs {
		m := provides
		if pkg.Name == "main" {
			m = binaries
		}
		m[path.Join(rootImportPath, pkgName)] = "//" + pkgName + ":" + pkg.Name
	}
	f, err := os.Create(path.Join(dir, "BUILD"))
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, pkgInfo{
		Name:     pkgName,
		Pkgs:     pkgs,
		Deps:     deps,
		Provides: provides,
		Binaries: binaries,
	})
}

type pkgInfo struct {
	Name               string
	Pkgs               map[string]*ast.Package
	Deps               []string
	Provides, Binaries map[string]string
}

func nonTestOnly(info os.FileInfo) bool {
	return !strings.HasSuffix(info.Name(), "_test.go")
}

var tmpl = template.Must(template.New("build").Funcs(template.FuncMap{
	"basename": func(s string) string { return path.Base(s) },
}).Parse(`
{{ range $pkgName, $pkg := .Pkgs }}
{{- if eq $pkgName "main" }}
go_binary(
{{- else }}
go_library(
{{- end }}
    name = "{{ $pkg.Name }}",
    srcs = [
        {{- range $src, $file := $pkg.Files }}
        "{{ basename $src }}",
        {{- end }}
    ],
    {{- if $.Deps }}
    deps = [
        {{- range $.Deps }}
        "{{ . }}",
        {{- end }}
    ],
    {{- end }}
    {{- if $pkg.Imports }}
    requires = [
        {{- range $path, $_ := $pkg.Imports }}
        "{{ $path }}",
        {{- end }}
    ],
    {{- end }}
    visibility = ["PUBLIC"],
)
{{ end }}

{{ if eq $.Name "" }}
filegroup(
    name = "_mod",
    provides = {
        {{- range $k, $v := $.Provides }}
        "{{ $k }}": "{{ $v }}",
        {{- end }}
    },
    visibility = ["PUBLIC"],
)

filegroup(
    name = "_bin",
    srcs = [
        {{- range $k, $v := $.Binaries }}
        "{{ $v }}",
        {{- end }}
    ],
    provides = {
        {{- range $k, $v := $.Binaries }}
        "{{ $k }}": "{{ $v }}",
        {{- end }}
    },
    visibility = ["PUBLIC"],
)
{{ end }}
`))
