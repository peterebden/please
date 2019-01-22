package provide

import (
	"io/ioutil"
	"path"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/thought-machine/please/third_party/go/vendor/modfile"
)

var versionRegex = regexp.MustCompile("v[0-9].0.0-[0-9]{14}-([0-9a-f]+)")

// A Module represents a single Go module.
type Module struct {
	Path, Version string
}

var modules = map[string]struct{}{}
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
	ret := make([]Module, 0, len(f.Require))
	for _, req := range f.Require {
		mod := Module(req.Mod)
		if repl, present := replacements[mod]; present {
			if repl.Path != "" {
				ret = append(ret, repl)
				addModule(filename, repl.Path)
			}
		} else {
			ret = append(ret, mod)
			addModule(filename, mod.Path)
		}
	}
	return ret, nil
}

// addModule marks a module as known. It takes a few guesses at possible locations.
func addModule(filename, mod string) {
	dir := path.Dir(filename)
	modules[mod] = struct{}{}
	modules[path.Join(dir, mod)] = struct{}{}
	modules[path.Join("plz-out/gen", dir, mod)] = struct{}{}
}

// isModule returns true if the given module is known.
func isModule(mod string) bool {
	mutex.Lock()
	defer mutex.Unlock()
	_, present := modules[mod]
	return present
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
	"version": func(in string) string {
		if match := versionRegex.FindStringSubmatch(in); match != nil {
			return match[1]
		}
		// In some cases versions are suffixed. We need to lose that suffix.
		return strings.TrimSuffix(in, "+incompatible")
	},
}).Parse(`
{{ range . }}
go_module(
    name = "{{ name .Path }}",
    path = "{{ .Path }}",
    version = "{{ version .Version }}",
)
{{ end }}
`))
