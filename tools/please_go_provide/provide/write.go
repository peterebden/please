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
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if !info.IsDir() || path == dir {
			return nil
		}
		return write(importPath, path[len(dir):], path, deps, provides)
	}); err != nil {
		return err
	}
	return write(importPath, "", dir, deps, provides)
}

// write writes a single BUILD file.
func write(rootImportPath, pkgName, dir string, deps []string, provides map[string]string) error {
	fs := token.NewFileSet()
	pkgs, err := parser.ParseDir(fs, dir, nonTestOnly, parser.ImportsOnly)
	if err != nil {
		return err
	}
	for _, pkg := range pkgs {
		provides[path.Join(rootImportPath, pkgName)] = "//" + pkgName + ":" + pkg.Name
	}
	f, err := os.Create(path.Join(dir, "BUILD"))
	if err != nil {
		return err
	}
	defer f.Close()
	info := pkgInfo{
		Pkgs: pkgs,
		Deps: deps,
	}
	if pkgName == "" {
		info.Provides = provides
	}
	return tmpl.Execute(f, info)
}

type pkgInfo struct {
	Pkgs     map[string]*ast.Package
	Deps     []string
	Provides map[string]string
}

func nonTestOnly(info os.FileInfo) bool {
	return !strings.HasSuffix(info.Name(), "_test.go")
}

var tmpl = template.Must(template.New("build").Funcs(template.FuncMap{
	"basename": func(s string) string { return path.Base(s) },
}).Parse(`
{{ range $pkgName, $pkg := .Pkgs }}
go_library(
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

{{ if $.Provides }}
filegroup(
    name = "module",
    provides = {
        {{- range $k, $v := $.Provides }}
        "{{ $k }}": "{{ $v }}",
        {{- end }}
    },
    visibility = ["PUBLIC"],
)
{{ end }}
`))
