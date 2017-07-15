package maven

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

const mavenJarTemplate = `maven_jar(
    name = '{{ .ArtifactId }}',
    id = '{{ .GroupId }}:{{ .ArtifactId }}:{{ .Version }}',
    hash = '',{{ if .Dependencies.Dependency }}
    deps = [
{{ range .Dependencies.Dependency }}        ':{{ .ArtifactId }}',
{{ end }}    ],{{ end }}
)`

// AllDependencies returns all the dependencies of this artifact in a short format
// that we consume later. The format is vaguely akin to a Maven id, although we consider
// it an internal detail - it must agree between this and the maven_jars build rule that
// consumes it, but we don't hold it stable between different Please versions. The format is:
// group_id:artifact_id:version:{src|no_src}[:licence|licence|...]
func AllDependencies(f *Fetch, id string, indent bool) []string {
	a := artifact{}
	if err := a.FromId(id); err != nil {
		log.Fatalf("%s\n", err)
	}
	pom := f.Pom(a)
	done := map[unversioned]bool{}
	indentIncrement := ""
	if indent {
		indentIncrement = "  "
	}
	// Just run through dependencies here, not the top-level pom itself.
	ret := []string{}
	for _, dep := range pom.AllDependencies() {
		if !done[dep.unversioned] {
			done[dep.unversioned] = true
			ret = append(ret, allDependencies(dep, "", indentIncrement, nil, done)...)
		}
	}
	return ret
}

// allDependencies implements the logic of AllDependencies with indenting.
func allDependencies(pom *pomXml, currentIndent, indentIncrement string, tmpl *template.Template, done map[unversioned]bool) []string {
	ret := []string{
		fmt.Sprintf("%s%s:%s", currentIndent, pom.Id(), source(pom)),
	}
	if tmpl != nil {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, pom); err != nil {
			log.Fatalf("%s\n", err)
		}
		ret[0] = buf.String()
	} else if licences := pom.AllLicences(); len(licences) > 0 {
		ret[0] += ":" + strings.Join(licences, "|")
	}
	for _, dep := range pom.AllDependencies() {
		if !done[dep.unversioned] {
			done[dep.unversioned] = true
			ret = append(ret, allDependencies(dep, currentIndent+indentIncrement, indentIncrement, tmpl, done)...)
		}
	}
	return ret
}

// source returns the src / no_src indicator for a single pom.
func source(pom *pomXml) string {
	if pom.HasSources {
		return "src"
	}
	return "no_src"
}

// BuildRules returns all the dependencies of this artifact as individual maven_jar build rules.
func BuildRules(f *Fetch, id string) []string {
	tmpl := template.Must(template.New("maven_jar").Parse(mavenJarTemplate))
	a := artifact{}
	if err := a.FromId(id); err != nil {
		log.Fatalf("%s\n", err)
	}
	return allDependencies(f.Pom(a), "", "", tmpl, map[unversioned]bool{})
}
