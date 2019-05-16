package asp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrFormat(t *testing.T) {
	s := &scope{
		locals: map[string]pyObject{},
	}
	assert.EqualValues(t, "test test test", strFormat(s, []pyObject{pyString("test test test")}))
	assert.EqualValues(t, "test blah test", strFormat(s, []pyObject{pyString("test {} test"), pyString("blah")}))
	assert.EqualValues(t, "test {} test", strFormat(s, []pyObject{pyString("test {{}} test"), pyString("blah")}))
	assert.EqualValues(t, "test ", strFormat(s, []pyObject{pyString("test {")}))
	assert.EqualValues(t, "test {", strFormat(s, []pyObject{pyString("test {{")}))
	s.Set("umpt", pyInt(42))
	s.Set("oof", None)
	assert.EqualValues(t, "test 42", strFormat(s, []pyObject{pyString("test {umpt}")}))
	assert.EqualValues(t, "test None 42", strFormat(s, []pyObject{pyString("test {} {umpt}"), None}))
}
