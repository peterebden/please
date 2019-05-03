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
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if !info.IsDir() {
			return nil
		}
		fs := token.NewFileSet()
		pkgs, err := parser.ParseDir(fs, path, nonTestOnly, parser.ImportsOnly)
		if err != nil {
			return err
		}
		f, err := os.Create(filepath.Join(path, "BUILD"))
		if err != nil {
			return err
		}
		defer f.Close()
		return tmpl.Execute(f, pkgInfo{Pkgs: pkgs, Deps: deps})
	})
}

type pkgInfo struct {
	Pkgs map[string]*ast.Package
	Deps []string
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
    visibility = ["PUBLIC"],
)
{{ end }}
`))
