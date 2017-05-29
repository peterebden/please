// Test to make sure we don't forget about adding new fields to print
// (because I keep doing that...)

package query

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"core"
)

// Add fields to this list *after* you teach print about them.
var KnownFields = map[string]bool{
	"BuildTimeout":                true,
	"BuildingDescription":         true,
	"Command":                     true,
	"Commands":                    true,
	"Containerise":                true,
	"ContainerSettings":           true,
	"Data":                        true,
	"dependencies":                true,
	"Flakiness":                   true,
	"Hashes":                      true,
	"IsBinary":                    true,
	"IsTest":                      true,
	"IsFilegroup":                 true,
	"Label":                       true, // this includes the target's name
	"Labels":                      true,
	"Licences":                    true,
	"namedOutputs":                true,
	"NamedSources":                true,
	"NeedsTransitiveDependencies": true,
	"NoTestOutput":                true,
	"OptionalOutputs":             true,
	"OutputIsComplete":            true,
	"outputs":                     true,
	"PreBuildFunction":            true,
	"PostBuildFunction":           true,
	"Provides":                    true,
	"Requires":                    true,
	"Sources":                     true,
	"Stamp":                       true,
	"TestCommand":                 true,
	"TestCommands":                true,
	"TestOnly":                    true,
	"TestOutputs":                 true,
	"TestTimeout":                 true,
	"Tools":                       true,
	"Visibility":                  true,

	// These aren't part of the declaration, only used internally.
	"state":         true,
	"Results":       true,
	"PreBuildHash":  true,
	"PostBuildHash": true,
	"RuleHash":      true,
	"mutex":         true,
}

func TestAllFieldsArePresentAndAccountedFor(t *testing.T) {
	target := core.BuildTarget{}
	val := reflect.ValueOf(target)
	for i := 0; i < val.Type().NumField(); i++ {
		field := val.Type().Field(i)
		if !KnownFields[field.Name] {
			t.Errorf("Unaccounted field in 'query print': %s", field.Name)
		}
	}
}

func TestPrintOutput(t *testing.T) {
	target := core.NewBuildTarget(core.ParseBuildLabel("//src/query:test_print_output", ""))
	target.AddSource(src("file.go"))
	target.AddSource(src(":target1"))
	target.AddSource(src("//src/query:target2"))
	target.AddSource(src("//src/query:target3|go"))
	target.AddSource(src("//src/core:core"))
	target.AddOutput("out1.go")
	target.AddOutput("out2.go")
	target.Command = "cp $SRCS $OUTS"
	target.Tools = append(target.Tools, src("//tools:tool1"))
	target.IsBinary = true
	s := testPrint(target)
	expected := `  build_rule(
      name = 'test_print_output',
      srcs = [
          'file.go',
          '//src/query:target1',
          '//src/query:target2',
          '//src/query:target3|go',
          '//src/core:core',
      ],
      outs = [
          'out1.go',
          'out2.go',
      ],
      cmd = 'cp $SRCS $OUTS',
      binary = True,
      tools = [
          '//tools:tool1',
      ],
  )

`
	assert.Equal(t, expected, s)
}

func TestFilegroupOutput(t *testing.T) {
	target := core.NewBuildTarget(core.ParseBuildLabel("//src/query:test_filegroup_output", ""))
	target.AddSource(src("file.go"))
	target.AddSource(src(":target1"))
	target.IsFilegroup = true
	target.Visibility = core.WholeGraph
	s := testPrint(target)
	expected := `  filegroup(
      name = 'test_filegroup_output',
      srcs = [
          'file.go',
          '//src/query:target1',
      ],
      visibility = [
          'PUBLIC',
      ],
  )

`
	assert.Equal(t, expected, s)
}

func TestTestOutput(t *testing.T) {
	target := core.NewBuildTarget(core.ParseBuildLabel("//src/query:test_test_output", ""))
	target.AddSource(src("file.go"))
	target.IsTest = true
	target.IsBinary = true
	target.BuildTimeout = 30 * time.Second
	target.TestTimeout = 60 * time.Second
	target.Flakiness = 2
	s := testPrint(target)
	expected := `  build_rule(
      name = 'test_test_output',
      srcs = [
          'file.go',
      ],
      binary = True,
      test = True,
      flaky = 2,
      timeout = 30,
      test_timeout = 60,
  )

`
	assert.Equal(t, expected, s)
}

func TestPostBuildOutput(t *testing.T) {
	target := core.NewBuildTarget(core.ParseBuildLabel("//src/query:test_post_build_output", ""))
	target.PostBuildFunction = 1
	target.AddCommand("opt", "/bin/true")
	target.AddCommand("dbg", "/bin/false")
	s := testPrint(target)
	expected := `  build_rule(
      name = 'test_post_build_output',
      cmd = {
          'opt': '/bin/true',
          'dbg': '/bin/false',
      },
      post_build = '<python ref>',
  )

`
	assert.Equal(t, expected, s)
}

func testPrint(target *core.BuildTarget) string {
	var buf bytes.Buffer
	p := printer{w: &buf}
	p.queryPrint(target)
	return buf.String()
}

func src(in string) core.BuildInput {
	const pkg = "src/query"
	if strings.HasPrefix(in, "//") || strings.HasPrefix(in, ":") {
		src, err := core.TryParseNamedOutputLabel(in, pkg)
		if err != nil {
			panic(err)
		}
		return src
	}
	return core.FileLabel{File: in, Package: pkg}
}
