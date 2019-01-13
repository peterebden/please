// Package main implements a simple tool for converting the output of 'go mod graph'
// into a BUILD file.
package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"text/template"

	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/cli"
)

var log = logging.MustGetLogger("mod_to_build")

var versionRegex = regexp.MustCompile("v[0-9].0.0-[0-9]{14}-([0-9a-f]+)")

// this is obviously awful, but I don't have a full-blown HTML parser handy.
var evilRegex = regexp.MustCompile(`<meta +name="go-import" +content="([^"]+)"`)

var tmpl = template.Must(template.New("").Parse(`
{{- range $_, $mod := . }}
{{- if $mod.Version }}
go_module(
    name = "{{ $mod.Name }}",
    path = "{{ $mod.Path }}",
    {{- with $mod.Repo }}
    repo = "{{ . }}",
    {{- end }}
    version = "{{ $mod.Version }}",
    {{- with $mod.Deps }}
    deps = [
    {{- range . }}
        ":{{ . }}",
    {{- end }}
    ],
    {{- end }}
)
{{- end }}
{{ end }}
`))

var opts = struct {
	Usage     string
	Verbosity cli.Verbosity     `short:"v" long:"verbosity" default:"notice" description:"Verbosity of output (higher number = more output)"`
	Names     map[string]string `short:"n" long:"name" description:"Mapping of package path -> name of build target to generate. Allows overriding given choices."`
}{
	Usage: `
mod_to_build is a simple tool for converting 'go mod' output to a BUILD file.

It simply invokes go mod graph so always looks at the module it is already in.

It doesn't necessarily produce perfect output at this point; there is of course the opportunity
to edit the produced BUILD file by hand though.
`,
}

type mod struct {
	Name, Path, Version, Repo string
	Deps                      []string
}

func split(s string, sep byte, must bool) (string, string) {
	if idx := strings.IndexByte(s, sep); idx != -1 {
		return s[:idx], s[idx+1:]
	} else if must {
		log.Fatalf("missing separator [%c] in %s", sep, s)
	}
	return s, ""
}

func determineRepo(modpath string) string {
	// Don't bother with these ones, saves a lot of downloading.
	if strings.HasPrefix(modpath, "github.com") || strings.HasPrefix(modpath, "golang.org") || strings.HasPrefix(modpath, "gopkg.in") {
		return ""
	}
	log.Notice("Determining repo path for %s", modpath)
	resp, err := http.Get("https://" + modpath + "?go-get=1")
	if err != nil {
		log.Error("Error looking up repo path for %s: %s", modpath, err)
		return ""
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("Error looking up repo path for %s: %s", modpath, err)
		return ""
	}
	match := evilRegex.FindStringSubmatch(string(b))
	if match == nil {
		log.Warning("Couldn't find go-import meta tag in response for %s, continuing.", modpath)
		return ""
	}
	parts := strings.Split(match[1], " ")
	if len(parts) != 3 {
		log.Warning("Unknown format of meta tag: %s", match[1])
		return ""
	}
	return strings.TrimPrefix(parts[2], "https://") // This is basically implicit later.
}

func main() {
	cli.ParseFlagsOrDie("mod_to_build", &opts)
	cli.InitLogging(opts.Verbosity)

	log.Notice("Analysing module graph...")
	cmd := exec.Command("go", "mod", "graph")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("Failed to run command: %s", err)
	}

	mods := map[string]*mod{}
	names := map[string]string{}
	for path, name := range opts.Names {
		names[name] = path
	}

	getMod := func(pkg string) *mod {
		modpath, version := split(pkg, '@', false)
		m := mods[modpath]
		if m == nil {
			// Handle a couple of foibles of their versioning scheme
			if match := versionRegex.FindStringSubmatch(version); match != nil {
				version = match[1]
			}
			m = &mod{
				Name:    path.Base(modpath),
				Path:    modpath,
				Repo:    determineRepo(modpath),
				Version: strings.TrimSuffix(version, "+incompatible"),
			}
			if name, present := opts.Names[modpath]; present {
				m.Name = name
			} else if names[m.Name] != "" {
				m.Name = path.Base(path.Dir(modpath)) + "_" + m.Name
			}
			names[m.Name] = m.Name
			mods[modpath] = m
		}
		return m
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pkg, dep := split(line, ' ', true)
		m := getMod(pkg)
		d := getMod(dep)
		m.Deps = append(m.Deps, d.Name)
	}

	if err := tmpl.Execute(os.Stdout, mods); err != nil {
		log.Fatalf("Error executing template: %s", err)
	}
}
