// Package provide implements basic Go BUILD file provisioning.
package provide

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strings"
	"text/template"
)

// Parse parses a single directory and returns a BUILD file for it.
func Parse(dir string) (string, error) {
	var b strings.Builder
	fs := token.NewFileSet()
	pkgs, err := parser.ParseDir(fs, dir, nil, parser.ImportsOnly)
	if err != nil {
		return "", err
	} else if err := tmpl.Execute(&b, pkgs); err != nil {
		return "", err
	}
	return b.String(), nil
}

var tmpl = template.Must(template.New("build").Funcs(template.FuncMap{
	"filter": func(in map[string]*ast.File, test bool) []string {
		ret := []string{}
		for name := range in {
			if strings.HasSuffix(name, "test.go") == test {
				ret = append(ret, path.Base(name))
			}
		}
		return ret
	},
}).Parse(`
{{ range $pkgName, $pkg := . }}
go_library(
    name = "{{ $pkgName }}",
    srcs = [
        {{ range filter $pkg.Files false }}
        "{{ . }}",
        {{ end }}
    ],
)

{{ if filter $pkg.Files true }}
go_test(
    name = "{{ $pkgName }}_test",
    srcs = [
        {{ range filter $pkg.Files true }}
        "{{ . }}",
        {{ end }}
    ],
    deps = [":{{ $pkgName }}"],
)
{{ end }}
{{ end }}
`))
