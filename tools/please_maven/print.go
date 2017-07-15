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
// group_id:artifact_id:version:{src|no_src}[:licence][:licence]...
func AllDependencies(f *Fetch, id string, indent bool) []string {
	a := artifact{}
	if err := a.FromId(id); err != nil {
		log.Fatalf("%s\n", err)
	}
	pom := f.Pom(a)
	if indent {
		return allDependencies(pom, "", "  ")
	}
	return allDependencies(pom, "", "")
}

// allDependencies implements the logic of AllDependencies with indenting.
func allDependencies(pom *pomXml, currentIndent, indentIncrement string) []string {
	ret := []string{
		fmt.Sprintf("%s%s:%s", currentIndent, pom.Id(), source(pom)),
	}
	if licences := pom.AllLicences(); len(licences) > 0 {
		ret[0] += ":" + strings.Join(licences, ":")
	}
	for _, dep := range pom.AllDependencies() {
		ret = append(ret, allDependencies(dep, currentIndent+indentIncrement, indentIncrement)...)
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
	deps := f.Pom(a).RecursiveDependencies()
	ret := make([]string, len(deps))
	for i, dep := range deps {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, dep); err != nil {
			log.Fatalf("%s\n", err)
		}
		ret[i] = buf.String()
	}
	return ret
}
