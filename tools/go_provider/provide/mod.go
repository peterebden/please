package provide

import (
	"io/ioutil"
	"path"
	"strings"
	"sync"
	"text/template"

	"github.com/thought-machine/please/third_party/go/vendor/modfile"
)

// A Module represents a single Go module.
type Module struct {
	Path, Version string
}

var replacements = map[Module]Module{}
var mutex sync.Mutex

// ParseMod parses a go.mod file into a series of modules.
func ParseMod(filename string) ([]Module, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	f, err := modfile.ParseLax(filename, data, nil)
	if err != nil {
		return nil, err
	}
	mutex.Lock()
	defer mutex.Unlock()
	for _, repl := range f.Replace {
		replacements[Module(repl.Old)] = Module(repl.New)
	}
	for _, excl := range f.Exclude {
		replacements[Module(excl.Mod)] = Module{}
	}
	ret := make([]Module, len(f.Require))
	for i, req := range f.Require {
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
	"name": func(in string) string { return path.Base(in) },
}).Parse(`
{{ range . }}
go_module(
    name = "{{ name .Path }}",
    path = "{{ .Path }}",
    version = "{{ .Version }}",
)
{{ end }}
`))
