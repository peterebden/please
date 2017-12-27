package participle

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseBasic(t *testing.T) {
	p := NewParser()
	statements, err := p.parse("src/parse/participle/test_data/basic.build")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(statements))
	assert.NotNil(t, statements[0].FuncDef)
	assert.Equal(t, "test", statements[0].FuncDef.Name)
	assert.Equal(t, 1, statements[0].FuncDef.Arguments)
	assert.Equal(t, "x", statements[0].FuncDef.Arguments[0].Name)
}
