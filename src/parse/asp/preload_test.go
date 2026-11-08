package asp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thought-machine/please/rules"
	"github.com/thought-machine/please/src/core"
)

const preloadedDefs = `
preloaded = "from preload"
shadowed = "from preload"
_private = "private from preload"
`

const preloadingBuildFile = `
seen = preloaded
private = _private
shadowed = "from package"
`

// interpretPreloadedPackages interprets a BUILD file in two packages, with a single preloaded subinclude.
func interpretPreloadedPackages(t *testing.T, parseMetadata bool) (*scope, *scope) {
	t.Helper()
	state := core.NewDefaultBuildState()
	state.ParseMetadata = parseMetadata

	label := core.BuildLabel{PackageName: "preload", Name: "defs"}
	target := core.NewBuildTarget(label)
	target.AddOutput("defs.build_defs")
	state.Graph.AddTarget(target)
	state.Config.Parse.PreloadSubincludes = []core.BuildLabel{label}
	require.NoError(t, os.MkdirAll(target.OutDir(), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target.OutDir(), "defs.build_defs"), []byte(preloadedDefs), 0o644))

	parser := NewParser(state, nil)
	src, err := rules.ReadAsset("builtins.build_defs")
	require.NoError(t, err)
	parser.MustLoadBuiltins("builtins.build_defs", src)
	statements, err := parser.ParseData([]byte(preloadingBuildFile), "BUILD")
	require.NoError(t, err)

	s1, err := parser.interpreter.interpretAll(context.Background(), core.NewPackage("pkg1"), nil, nil, statements)
	require.NoError(t, err)
	s2, err := parser.interpreter.interpretAll(context.Background(), core.NewPackage("pkg2"), nil, nil, statements)
	require.NoError(t, err)
	return s1, s2
}

func assertPreloadedValues(t *testing.T, s *scope) {
	t.Helper()
	assert.Equal(t, pyString("from preload"), s.Lookup("seen"))
	assert.Equal(t, pyString("private from preload"), s.Lookup("private"))
	assert.Equal(t, pyString("from package"), s.Lookup("shadowed"))
}

func TestPreloadedSymbolsAreSharedBetweenPackages(t *testing.T) {
	s1, s2 := interpretPreloadedPackages(t, false)
	assertPreloadedValues(t, s1)
	assertPreloadedValues(t, s2)

	assert.Same(t, s1.parent, s2.parent)
	assert.NotSame(t, s1.interpreter.scope, s1.parent)
	assert.Same(t, s1.interpreter.scope, s1.parent.parent)
	assert.Nil(t, s1.LocalLookup("preloaded"), "preloaded symbols shouldn't be copied into the package")

	// Shadowing a symbol in one package must not be visible to anything else using the shared scope.
	assert.Equal(t, pyString("from preload"), s1.parent.Lookup("shadowed"))
	assert.Nil(t, s1.parent.LocalLookup("CONFIG"))
}

func TestPreloadedSymbolsAreCopiedWhenTrackingMetadata(t *testing.T) {
	s1, s2 := interpretPreloadedPackages(t, true)
	assertPreloadedValues(t, s1)
	assertPreloadedValues(t, s2)

	assert.Same(t, s1.interpreter.scope, s1.parent)
	assert.NotNil(t, s1.LocalLookup("preloaded"), "symbols must be in the package's own scope to track their origin")
}
