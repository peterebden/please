package plugin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thought-machine/please/src/plugin"
)

func TestLoadPlugin(t *testing.T) {
	// Sanity check that the produced plugin is importable
	register := plugin.MustLoadSymbol[func()]("prometheus", "Register")
	assert.NotNil(t, register)
	register()
}
