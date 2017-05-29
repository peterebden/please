package query

import (
	"fmt"
	"io"
	"os"

	"core"
)

// QueryPrint produces a Python call which would (hopefully) regenerate the same build rule if run.
// This is of course not ideal since they were almost certainly created as a java_library
// or some similar wrapper rule, but we've lost that information by now.
func QueryPrint(graph *core.BuildGraph, labels []core.BuildLabel) {
	for _, label := range labels {
		fmt.Fprintf(os.Stderr, "%s:\n", label)
		p := printer{w: os.Stdout}
		p.queryPrint(graph.TargetOrDie(label))
	}
}

// A printer is responsible for creating the output of 'plz query print'.
type printer struct {
	w io.Writer
}

func (p *printer) printf(msg string, args ...interface{}) {
	fmt.Fprintf(p.w, msg, args...)
}

func (p *printer) queryPrint(target *core.BuildTarget) {
	if target.IsFilegroup {
		p.printf("  filegroup(\n")
	} else {
		p.printf("  build_rule(\n")
	}
	p.printf("      name = '%s',\n", target.Label.Name)
	if len(target.Sources) > 0 {
		p.printf("      srcs = [\n")
		for _, src := range target.Sources {
			p.printf("          '%s',\n", src)
		}
		p.printf("      ],\n")
	} else if target.NamedSources != nil {
		p.printf("      srcs = {\n")
		for name, srcs := range target.NamedSources {
			p.printf("          '%s': [\n", name)
			for _, src := range srcs {
				p.printf("              '%s'\n", src)
			}
			p.printf("          ],\n")
		}
		p.printf("      },\n")
	}
	if len(target.DeclaredOutputs()) > 0 && !target.IsFilegroup {
		p.printf("      outs = [\n")
		for _, out := range target.DeclaredOutputs() {
			p.printf("          '%s',\n", out)
		}
		p.printf("      ],\n")
	} else if names := target.DeclaredOutputNames(); len(names) > 0 {
		p.printf("      outs = {\n")
		outs := target.DeclaredNamedOutputs()
		for _, name := range names {
			p.printf("          '%s': [\n", name)
			for _, out := range outs[name] {
				p.printf("              '%s'\n", out)
			}
			p.printf("          ],\n")
		}
		p.printf("      },\n")
	}
	p.stringList("optional_outs", target.OptionalOutputs)
	if !target.IsFilegroup {
		if target.Command == "" {
			p.pythonDict(target.Commands, "cmd")
		} else {
			p.printf("      cmd = '%s',\n", target.Command)
		}
	}
	p.pythonDict(target.TestCommands, "test_cmd")
	if target.TestCommand != "" {
		p.printf("      test_cmd = '%s',\n", target.TestCommand)
	}
	p.pythonBool("binary", target.IsBinary)
	p.pythonBool("test", target.IsTest)
	p.pythonBool("needs_transitive_deps", target.NeedsTransitiveDependencies)
	if !target.IsFilegroup {
		p.pythonBool("output_is_complete", target.OutputIsComplete)
		if target.BuildingDescription != core.DefaultBuildingDescription {
			p.printf("      building_description = '%s',\n", target.BuildingDescription)
		}
	}
	p.pythonBool("stamp", target.Stamp)
	if target.ContainerSettings != nil {
		p.printf("      container = {\n")
		p.printf("          'docker_image': '%s',\n", target.ContainerSettings.DockerImage)
		p.printf("          'docker_user': '%s',\n", target.ContainerSettings.DockerUser)
		p.printf("          'docker_run_args': '%s',\n", target.ContainerSettings.DockerRunArgs)
	} else {
		p.pythonBool("container", target.Containerise)
	}
	p.pythonBool("no_test_output", target.NoTestOutput)
	p.pythonBool("test_only", target.TestOnly)
	p.labelList("deps", excludeLabels(target.DeclaredDependencies(), target.ExportedDependencies(), sourceLabels(target)), target)
	p.labelList("exported_deps", target.ExportedDependencies(), target)
	if len(target.Tools) > 0 {
		p.printf("      tools = [\n")
		for _, tool := range target.Tools {
			p.printf("          '%s',\n", tool)
		}
		p.printf("      ],\n")
	}
	if len(target.Data) > 0 {
		p.printf("      data = [\n")
		for _, datum := range target.Data {
			p.printf("          '%s',\n", datum)
		}
		p.printf("      ],\n")
	}
	p.stringList("labels", excludeStrings(target.Labels, target.Requires))
	p.stringList("hashes", target.Hashes)
	p.stringList("licences", target.Licences)
	p.stringList("test_outputs", target.TestOutputs)
	p.stringList("requires", target.Requires)
	if len(target.Provides) > 0 {
		p.printf("      provides = {\n")
		for k, v := range target.Provides {
			if v.PackageName == target.Label.PackageName {
				p.printf("          '%s': ':%s',\n", k, v.Name)
			} else {
				p.printf("          '%s': '%s',\n", k, v)
			}
		}
		p.printf("      },\n")
	}
	if target.Flakiness > 0 {
		p.printf("      flaky = %d,\n", target.Flakiness)
	}
	if target.BuildTimeout > 0 {
		p.printf("      timeout = %0.0f,\n", target.BuildTimeout.Seconds())
	}
	if target.TestTimeout > 0 {
		p.printf("      test_timeout = %0.0f,\n", target.TestTimeout.Seconds())
	}
	if len(target.Visibility) > 0 {
		p.printf("      visibility = [\n")
		for _, vis := range target.Visibility {
			if vis.PackageName == "" && vis.IsAllSubpackages() {
				p.printf("          'PUBLIC',\n")
			} else {
				p.printf("          '%s',\n", vis)
			}
		}
		p.printf("      ],\n")
	}
	if target.PreBuildFunction != 0 {
		p.printf("      pre_build = '<python ref>',\n") // Don't have any sensible way of printing this.
	}
	if target.PostBuildFunction != 0 {
		p.printf("      post_build = '<python ref>',\n") // Don't have any sensible way of printing this.
	}
	p.printf("  )\n\n")
}

func (p *printer) pythonBool(s string, b bool) {
	if b {
		p.printf("      %s = True,\n", s)
	}
}

func (p *printer) pythonDict(m map[string]string, name string) {
	if m != nil {
		p.printf("      %s = {\n", name)
		for config, command := range m {
			p.printf("          '%s': '%s',\n", config, command)
		}
		p.printf("      },\n")
	}
}

func (p *printer) labelList(s string, l []core.BuildLabel, target *core.BuildTarget) {
	if len(l) > 0 {
		p.printf("      %s = [\n", s)
		for _, d := range l {
			p.printLabel(d, target)
		}
		p.printf("      ],\n")
	}
}

// printLabel prints a single label relative to a given target.
func (p *printer) printLabel(label core.BuildLabel, target *core.BuildTarget) {
	if label.PackageName == target.Label.PackageName {
		p.printf("          ':%s',\n", label.Name)
	} else {
		p.printf("          '%s',\n", label)
	}

}

func (p *printer) stringList(s string, l []string) {
	if len(l) > 0 {
		p.printf("      %s = [\n", s)
		for _, d := range l {
			p.printf("          '%s',\n", d)
		}
		p.printf("      ],\n")
	}
}

// excludeLabels returns a filtered slice of labels from l that are not in excl.
func excludeLabels(l []core.BuildLabel, excl ...[]core.BuildLabel) []core.BuildLabel {
	var ret []core.BuildLabel
	// This is obviously quadratic but who cares, the lists will not be long.
outer:
	for _, x := range l {
		for _, y := range excl {
			for _, z := range y {
				if x == z {
					continue outer
				}
			}
		}
		ret = append(ret, x)
	}
	return ret
}

// excludeStrings returns a filtered slice of strings from l that are not in excl.
func excludeStrings(l, excl []string) []string {
	var ret []string
outer:
	for _, x := range l {
		for _, y := range excl {
			if x == y {
				continue outer
			}
		}
		ret = append(ret, x)
	}
	return ret
}

// sourceLabels returns all the labels that are sources of this target.
func sourceLabels(target *core.BuildTarget) []core.BuildLabel {
	ret := make([]core.BuildLabel, 0, len(target.Sources))
	for _, src := range target.Sources {
		if src.Label() != nil {
			ret = append(ret, *src.Label())
		}
	}
	return ret
}
