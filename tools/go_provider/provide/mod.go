package provide

import (
	"fmt"
	"go/scanner"
	"go/token"
	"io/ioutil"
	"strings"
	"sync"
	"text/template"
)

// A Module represents a single Go module.
type Module struct {
	Path, Version string
}

var replacements = map[Module]Module{}
var mutex sync.Mutex

// ParseMod parses a go.mod file into a series of modules.
func ParseMod(filename string) ([]Module, error) {
	// We have to parse this ourselves, there is no publicly visible parser for these files :(
	p := parser{}
	require, replace, exclude, err := p.ParseAll(filename)
	mutex.Lock()
	defer mutex.Unlock()
	for k, v := range replace {
		replacements[k] = v
	}
	for _, excl := range exclude {
		replacements[excl] = Module{}
	}
	ret := make([]Module, len(f.Require))
	for _, mod := range require {
		mod := Module(req.Mod)
		if repl, present := replacements[mod]; present {
			if repl.Path != "" {
				ret[i] = repl
			}
		} else {
			ret[i] = mod
		}
	}
	return ret, nil
}

// Provide converts a go.mod file into a BUILD file.
func ProvideMod(filename string) (string, error) {
	var b strings.Builder
	mods, err := ParseMod(filename)
	if err != nil {
		return "", err
	}
	err = modTmpl.Execute(&b, mods)
	return b.String(), err
}

var modTmpl = template.Must(template.New("build").Funcs(template.FuncMap{
	"name": func(in string) string { return strings.Replace(in, "/", "_", -1) },
}).Parse(`
{{ range . }}
go_module(
    name = "{{ name . }}",
    path = "{{ .Path }}",
    version = "{{ .Version }}",
)
{{ end }}
`))

type parser struct {
	s scanner.Scanner
}

func (p *parser) ParseAll(filename string) (require []Module, replace map[Module]Module, exclude []Module, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s", r)
		}
	}()
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}
	fs := token.NewFileSet()
	f := fs.AddFile(filename, 0, 0)
	p.s.Init(f, b, nil, 0)
	for {
		if pos, tok, lit := s.Scan(); tok == token.EOF {
			break
		} else if lit == "module" {
			p.parseOne()
		} else if lit == "require" {
			require = p.parseList()
		} else if lit == "replace" {
			replace = p.parseMap()
		} else if lit == "exclude" {
			exclude = p.parseList()
		} else {
			err = fmt.Errorf("Unknown statement in file: %s", lit)
		}
	}
}

func (p *parser) parseOne() string {

}
