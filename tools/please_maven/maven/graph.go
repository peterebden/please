package maven

import (
	"strings"
)

// A Graph is a minimal representation of the parts of `plz query graph`'s output that we care about.
type Graph struct {
	Packages map[string]struct {
		Targets map[string]struct {
			Labels []string `json:"labels,omitempty"`
		} `json:"targets"`
	} `json:"packages"`
	mavenToPackage map[string]string
}

// BuildMapping sets up the internal reverse mapping of maven id -> target.
// It must be called once before anything else is.
func (g *Graph) BuildMapping() {
	g.mavenToPackage = map[string]string{}
	for pkgName, pkg := range g.Packages {
		for targetName, target := range g.Targets {
			for _, label := range target.Labels {
				if parts := strings.Split(label, ":"); len(parts) > 3 && parts[0] == "mvn" {
					g.mavenToPackage[parts[1]+":"+parts[2]] = "//" + pkgName + ":" + targetName
				}
			}
		}
	}
}

// Needed returns true if we need a build rule for the given group ID / artifact ID.
// It's false if one already exists in the current build files.
func (g *Graph) Needed(groupID, artifactID string) bool {
	return g.MavenToPackage[groupID+":"+artifactID] != ""
}

// Dep returns the dependency for a given groupID / artifact ID.
