package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSVGPath(t *testing.T) {
	const expected = "M400,100 L400,0 A400,400 1 0,1 800.00000,400.00000 L700.00000,400.00000 A300,300 1 0,0 400,100"
	actual := svgPath(0, 4611686018427387904)
	assert.Equal(t, expected, actual)
	actual = svgPath(4611686018427387904, 9223372036854775808)
	assert.Equal(t, expected, actual)
}
