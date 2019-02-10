// Package provide implements basic Go BUILD file provisioning.
package provide

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strings"
	"text/template"
)

// ProvideDir parses a single directory and returns a BUILD file for it.
func ProvideDir(dir string) (string, error) {
	var b strings.Builder
	fs := token.NewFileSet()
	pkgs, err := parser.ParseDir(fs, dir, nil, parser.ImportsOnly)
	if err != nil {
		return "", err
	}
	// Name the targets after the directory, not the package name which is not predictable.
	for _, pkg := range pkgs {
		pkg.Name = path.Base(dir)
		break
	}
	if err := tmpl.Execute(&b, pkgs); err != nil {
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
		sort.Strings(ret)
		return ret
	},
}).Parse(`
{{ range $pkgName, $pkg := . }}
{{ with filter $pkg.Files false }}
go_library(
    name = "{{ $pkg.Name }}",
    srcs = [
        {{- range . }}
        "{{ . }}",
        {{- end }}
    ],
    visibility = ["PUBLIC"],
)
{{ end }}
{{ with filter $pkg.Files true }}
go_test(
    name = "{{ $pkg.Name }}_test",
    srcs = [
        {{- range . }}
        "{{ . }}",
        {{- end }}
    ],
    deps = [":{{ $pkgName }}"],
)
{{ end }}
{{ end }}
`))
