// Package provide implements provisioning of BUILD files for go_module subrepos.
package provide

import (
	"fmt"
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
	if err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if !info.IsDir() || p == dir || path.Base(dir) == "testdata" {
			return nil
		}
		return write(importPath, strings.Trim(p[len(dir):], "/"), p, deps, provides, binaries)
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
		fmt.Fprintf(os.Stderr, "Failed to parse Go files in %s: %s\n", pkgName, err)
		return nil // Don't die fatally; otherwise we are at the mercy of any one bad file in any repo.
	}
	ourpkgs := map[string]*pkgInfo{}
	for name, pkg := range pkgs {
		m := provides
		if pkg.Name == "main" {
			m = binaries
		}
		m[path.Join(rootImportPath, pkgName)] = "//" + pkgName + ":" + pkg.Name
		ourpkg := &pkgInfo{Pkg: pkg}
		ourpkgs[name] = ourpkg
		for _, file := range pkg.Files {
			for _, imp := range file.Imports {
				p := strings.Trim(imp.Path.Value, `"`)
				if strings.HasPrefix(p, rootImportPath) {
					ourpkg.LocalDeps = append(ourpkg.LocalDeps, "//"+strings.TrimLeft(p[len(rootImportPath):], "/")+":"+path.Base(p))
				}
				if strings.Contains(p, ".") { // quick and dirty way of not adding stdlib
					ourpkg.Imports = append(ourpkg.Imports, p)
				}
			}
		}
	}
	f, err := os.Create(path.Join(dir, "BUILD"))
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, pkgsInfo{
		Name:       pkgName,
		Dir:        dir,
		ImportPath: rootImportPath,
		Pkgs:       ourpkgs,
		Deps:       deps,
		Provides:   provides,
		Binaries:   binaries,
	})
}

type pkgsInfo struct {
	Name, ImportPath, Dir string
	Pkgs                  map[string]*pkgInfo
	Deps                  []string
	Provides, Binaries    map[string]string
}

type pkgInfo struct {
	Pkg       *ast.Package
	LocalDeps []string
	Imports   []string
}

func nonTestOnly(info os.FileInfo) bool {
	return !strings.HasSuffix(info.Name(), "_test.go")
}

var tmpl = template.Must(template.New("build").Funcs(template.FuncMap{
	"basename": func(s string) string { return path.Base(s) },
}).Parse(`
package(go_import_path = "{{ .ImportPath }}")
{{ range $pkgName, $pkg := .Pkgs }}
{{ if eq $pkgName "main" }}
go_binary(
    name = "{{ $pkgName }}",
{{ else }}
go_library(
    name = "{{ basename $.Dir }}",
{{- end }}
    srcs = [
        {{- range $src, $file := $pkg.Pkg.Files }}
        "{{ basename $src }}",
        {{- end }}
    ],
    {{- if or $.Deps $pkg.LocalDeps }}
    deps = [
        {{- range $pkg.LocalDeps }}
        "{{ . }}",
        {{- end }}
        {{- range $.Deps }}
        "@{{ . }}",
        {{- end }}
    ],
    {{- end }}
    {{- if $pkg.Imports }}
    _requires = [
        {{- range $pkg.Imports }}
        "{{ . }}",
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