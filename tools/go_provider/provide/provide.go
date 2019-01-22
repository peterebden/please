// Package provide implements basic Go BUILD file provisioning.
package provide

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
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
	data := struct {
		Pkgs map[string]*ast.Package
		Deps []string
	}{
		Pkgs: pkgs,
	}
	if isModule(dir) {
		deps, err := findDeps(dir)
		if err != nil {
			return "", err
		}
		data.Deps = deps
	}
	if err := tmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

// findDeps finds all the dependencies of a top-level module, i.e. all the subdirectories.
func findDeps(dir string) ([]string, error) {
	files := map[string]bool{}
	ret := []string{}
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "test.go") {
			p := filepath.Dir(path[len(dir)+1:])
			if p == "." || p == "/" {
				p = ":" + filepath.Base(dir)
			}
			if !files[p] {
				files[p] = true
				ret = append(ret, "//"+p)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(ret)
	return ret, nil
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
{{ if .Deps }}
filegroup(
    name = "mod",
    deps = [
        {{- range .Deps }}
        "{{ . }}",
        {{- end}}
    ],
    output_is_complete = False,
    visibility = ["PUBLIC"],
)
{{ end }}

{{ range $pkgName, $pkg := .Pkgs }}
go_library(
    name = "{{ $pkg.Name }}",
    srcs = [
        {{- range filter $pkg.Files false }}
        "{{ . }}",
        {{- end }}
    ],
    visibility = ["PUBLIC"],
)

{{ if filter $pkg.Files true }}
go_test(
    name = "{{ $pkg.Name }}_test",
    srcs = [
        {{- range filter $pkg.Files true }}
        "{{ . }}",
        {{- end }}
    ],
    deps = [":{{ $pkgName }}"],
)
{{ end }}
{{ end }}
`))
