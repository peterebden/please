package maven

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	"gopkg.in/op/go-logging.v1"
)

var log = logging.MustGetLogger("maven")

type unversioned struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
}

type Artifact struct {
	unversioned
	Version  string `xml:"version"`
	isParent bool
}

// GroupPath returns the group ID as a path.
func (a *Artifact) GroupPath() string {
	return strings.Replace(a.GroupId, ".", "/", -1)
}

// MetadataPath returns the path to the metadata XML file for this artifact.
func (a *Artifact) MetadataPath() string {
	return a.GroupPath() + "/" + a.ArtifactId + "/maven-metadata.xml"
}

// Path returns the path to an artifact that we'd download.
func (a *Artifact) Path(suffix string) string {
	return a.GroupPath() + "/" + a.ArtifactId + "/" + a.Version + "/" + a.ArtifactId + "-" + a.Version + suffix
}

// PomPath returns the path to the pom.xml for this artifact.
func (a *Artifact) PomPath() string {
	return a.Path(".pom")
}

// SourcePath returns the path to the sources jar for this artifact.
func (a *Artifact) SourcePath() string {
	return a.Path("-sources.jar")
}

// Id returns a Maven identifier for this artifact (i.e. GroupId:ArtifactId:Version)
func (a *Artifact) Id() string {
	return a.GroupId + ":" + a.ArtifactId + ":" + a.Version
}

// FromId loads this artifact from a Maven id.
func (a *Artifact) FromId(id string) error {
	split := strings.Split(id, ":")
	if len(split) != 3 {
		return fmt.Errorf("Invalid Maven artifact id %s; must be in the form group:artifact:version", id)
	}
	a.GroupId = split[0]
	a.ArtifactId = split[1]
	a.Version = split[2]
	return nil
}

// UnmarshalFlag implements the flags.Unmarshaler interface.
// This lets us use Artifact instances directly as flags.
func (a *Artifact) UnmarshalFlag(value string) error {
	return a.FromId(value)
}

type pomProperty struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type pomXml struct {
	Artifact
	Dependencies         pomDependencies `xml:"dependencies"`
	DependencyManagement struct {
		Dependencies pomDependencies `xml:"dependencies"`
	} `xml:"dependencyManagement"`
	Properties struct {
		Property []pomProperty `xml:",any"`
	} `xml:"properties"`
	Licences struct {
		Licence []struct {
			Name string `xml:"name"`
		} `xml:"license"`
	} `xml:"licenses"`
	Parent        Artifact `xml:"parent"`
	PropertiesMap map[string]string
	HasSources    bool
}

type pomDependency struct {
	Artifact
	Pom      *pomXml
	Scope    string `xml:"scope"`
	Optional bool   `xml:"optional"`
	// TODO(pebers): Handle exclusions here.
}

type pomDependencies struct {
	Dependency []*pomDependency `xml:"dependency"`
}

type mavenMetadataXml struct {
	Version    string `xml:"version"`
	Versioning struct {
		Latest   string `xml:"latest"`
		Release  string `xml:"release"`
		Versions struct {
			Version []string `xml:"version"`
		} `xml:"versions"`
	} `xml:"versioning"`
	Group, Artifact string
}

// LatestVersion returns the latest available version of a package
func (metadata *mavenMetadataXml) LatestVersion() string {
	if metadata.Versioning.Release != "" {
		return metadata.Versioning.Release
	} else if metadata.Versioning.Latest != "" {
		log.Warning("No release version for %s:%s, using latest", metadata.Group, metadata.Artifact)
		return metadata.Versioning.Latest
	} else if metadata.Version != "" {
		log.Warning("No release version for %s:%s", metadata.Group, metadata.Artifact)
		return metadata.Version
	}
	log.Fatalf("Can't find a version for %s:%s", metadata.Group, metadata.Artifact)
	return ""
}

// HasVersion returns true if the given package has the specified version.
func (metadata *mavenMetadataXml) HasVersion(version string) bool {
	for _, v := range metadata.Versioning.Versions.Version {
		if v == version {
			return true
		}
	}
	return false
}

// Unmarshal reads a metadata object from raw XML. It dies on any error.
func (metadata *mavenMetadataXml) Unmarshal(content []byte) {
	if err := xml.Unmarshal(content, metadata); err != nil {
		log.Fatalf("Error parsing metadata XML: %s\n", err)
	}
}

// AddProperty adds a property (typically from a parent or wherever), without overwriting.
func (pom *pomXml) AddProperty(property pomProperty) {
	if _, present := pom.PropertiesMap[property.XMLName.Local]; !present {
		pom.PropertiesMap[property.XMLName.Local] = property.Value
		pom.Properties.Property = append(pom.Properties.Property, property)
	}
}

// replaceVariables a Maven variable in the given string.
func (pom *pomXml) replaceVariables(s string) string {
	if strings.HasPrefix(s, "${") {
		if prop, present := pom.PropertiesMap[s[2:len(s)-1]]; !present {
			log.Fatalf("Failed property lookup %s: %s\n", s, pom.PropertiesMap)
		} else {
			return prop
		}
	}
	return s
}

// Unmarshal parses a downloaded pom.xml. This is of course less trivial than you would hope.
func (pom *pomXml) Unmarshal(f *Fetch, response []byte) {
	artifact := (*pom).Artifact // Copy this for later
	// This is an absolutely awful hack; we should use a proper decoder, but that seems
	// to be provoking a panic from the linker for reasons I don't fully understand right now.
	response = bytes.Replace(response, []byte("encoding=\"ISO-8859-1\""), []byte{}, -1)
	if err := xml.Unmarshal(response, pom); err != nil {
		log.Fatalf("Error parsing XML response: %s\n", err)
	}
	// Clean up strings in case they have spaces
	pom.GroupId = strings.TrimSpace(pom.GroupId)
	pom.ArtifactId = strings.TrimSpace(pom.ArtifactId)
	pom.Version = strings.TrimSpace(pom.Version)
	for i, licence := range pom.Licences.Licence {
		pom.Licences.Licence[i].Name = strings.TrimSpace(licence.Name)
	}
	// Handle properties nonsense, because of course it doesn't work this out for us...
	pom.PropertiesMap = map[string]string{}
	for _, prop := range pom.Properties.Property {
		pom.PropertiesMap[prop.XMLName.Local] = prop.Value
	}
	// There are also some properties that aren't described by the above - "project" is a bit magic.
	pom.PropertiesMap["groupId"] = pom.GroupId
	pom.PropertiesMap["artifactId"] = pom.ArtifactId
	pom.PropertiesMap["version"] = pom.Version
	pom.PropertiesMap["project.groupId"] = pom.GroupId
	pom.PropertiesMap["project.version"] = pom.Version
	if pom.Parent.ArtifactId != "" {
		if pom.Parent.GroupId == pom.GroupId && pom.Parent.ArtifactId == pom.ArtifactId {
			log.Fatalf("Circular dependency: %s:%s:%s specifies itself as its own parent", pom.GroupId, pom.ArtifactId, pom.Version)
		}
		// Must inherit variables from the parent.
		pom.Parent.isParent = true
		parent := f.Pom(&pom.Parent)
		for _, prop := range parent.Properties.Property {
			pom.AddProperty(prop)
		}
	}
	pom.Version = pom.replaceVariables(pom.Version)
	// Sanity check, but must happen after we resolve variables.
	if (pom.GroupId != "" && artifact.GroupId != pom.GroupId) ||
		(pom.ArtifactId != "" && artifact.ArtifactId != pom.ArtifactId) ||
		(pom.Version != "" && artifact.Version != "" && artifact.Version != pom.Version) {
		// These are a bit fiddly since inexplicably the fields are sometimes empty.
		log.Fatalf("Bad artifact: expected %s:%s:%s, got %s:%s:%s\n", artifact.GroupId, artifact.ArtifactId, artifact.Version, pom.GroupId, pom.ArtifactId, pom.Version)
	}
	// Arbitrarily, some pom files have this different structure with the extra "dependencyManagement" level.
	pom.Dependencies.Dependency = append(pom.Dependencies.Dependency, pom.DependencyManagement.Dependencies.Dependency...)
	pom.HasSources = f.HasSources(&pom.Artifact)
	if !pom.isParent {
		for _, dep := range pom.Dependencies.Dependency {
			pom.fetchDependency(f, dep)
		}
	}
}

func (pom *pomXml) fetchDependency(f *Fetch, dep *pomDependency) {
	// This is a bit of a hack; our build model doesn't distinguish these in the way Maven does.
	// TODO(pebers): Consider allowing specifying these to this tool to produce test-only deps.
	// Similarly system deps don't actually get fetched from Maven.
	if dep.Scope == "test" || dep.Scope == "system" {
		log.Debug("Not fetching %s:%s because of scope", dep.GroupId, dep.ArtifactId)
		return
	}
	if dep.Optional && !f.ShouldInclude(dep.ArtifactId) {
		log.Debug("Not fetching optional dependency %s:%s", dep.GroupId, dep.ArtifactId)
		return
	}
	log.Debug("Fetching %s (depended on by %s)", dep.Id(), pom.Id())
	dep.GroupId = pom.replaceVariables(dep.GroupId)
	dep.ArtifactId = pom.replaceVariables(dep.ArtifactId)
	// Not sure what this is about; httpclient seems to do this. It seems completely unhelpful but
	// no doubt there's some highly obscure case where it's considered useful.
	pom.PropertiesMap[dep.ArtifactId+".version"] = ""
	pom.PropertiesMap[strings.Replace(dep.ArtifactId, "-", ".", -1)+".version"] = ""
	dep.Version = strings.Trim(pom.replaceVariables(dep.Version), "[]")
	if strings.Contains(dep.Version, ",") {
		log.Fatalf("Can't do dependency mediation for %s", dep.Id())
	}
	if f.IsExcluded(dep.ArtifactId) {
		return
	}
	if dep.Version == "" {
		// If no version is specified, we can take any version that we've already found.
		if pom := f.Resolver.Pom(&dep.Artifact); pom != nil {
			dep.Pom = pom
			return
		}

		// Not 100% sure what the logic should really be here; for example, jacoco
		// seems to leave these underspecified and expects the same version, but other
		// things seem to expect the latest. Most likely it is some complex resolution
		// logic, but we'll take a stab at the same if the group matches and the same
		// version exists, otherwise we'll take the latest.
		if metadata := f.Metadata(&dep.Artifact); dep.GroupId == pom.GroupId && metadata.HasVersion(pom.Version) {
			dep.Version = pom.Version
		} else {
			dep.Version = metadata.LatestVersion()
		}
	}
	dep.Pom = f.Pom(&dep.Artifact)
}

// AllDependencies returns all the dependencies for this package.
func (pom *pomXml) AllDependencies() []*pomXml {
	deps := make([]*pomXml, 0, len(pom.Dependencies.Dependency))
	for _, dep := range pom.Dependencies.Dependency {
		if dep.Pom != nil {
			deps = append(deps, dep.Pom)
		}
	}
	return deps
}

// AllLicences returns all the licences for this package.
func (pom *pomXml) AllLicences() []string {
	licences := make([]string, len(pom.Licences.Licence))
	for i, licence := range pom.Licences.Licence {
		licences[i] = licence.Name
	}
	return licences
}
