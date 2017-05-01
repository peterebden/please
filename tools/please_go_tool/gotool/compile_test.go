package gotool

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplaceEnv(t *testing.T) {
	os.Setenv("PKG", "tools/please_go_tool/gotool")
	os.Setenv("TMP_DIR", "/tmp/please/plz-out/tmp/gotool")
	expected := "/tmp/please/plz-out/tmp/gotool/tools/please_go_tool/gotool"
	assert.Equal(t, expected, ReplaceEnv("$TMP_DIR/$PKG"))
	expected = "/tmp/please/plz-out/tmp/gotool/tools/please_go_tool/gotool/third_party/go"
	assert.Equal(t, expected, ReplaceEnv("$TMP_DIR/$PKG/third_party/go"))
	expected = "/tmp/please/plz-out/tmp/gotool/tools/please_go_tool/gotool/third_party/go"
	assert.Equal(t, expected, ReplaceEnv("${TMP_DIR}/${PKG}/third_party/go"))
}
