package core

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIncludes(t *testing.T) {
	label1 := BuildLabel{PackageName: "src/core", Name: "..."}
	label2 := BuildLabel{PackageName: "src/parse", Name: "parse"}
	assert.False(t, label1.Includes(label2))
	label2 = BuildLabel{PackageName: "src/core", Name: "core_test"}
	assert.True(t, label1.Includes(label2))
}

func TestIncludesSubstring(t *testing.T) {
	label1 := BuildLabel{PackageName: "third_party/python", Name: "..."}
	label2 := BuildLabel{PackageName: "third_party/python3", Name: "six"}
	assert.False(t, label1.Includes(label2))
}

func TestIncludesSubpackages(t *testing.T) {
	label1 := BuildLabel{PackageName: "", Name: "..."}
	label2 := BuildLabel{PackageName: "third_party/python3", Name: "six"}
	assert.True(t, label1.Includes(label2))
}

func TestParent(t *testing.T) {
	label := BuildLabel{PackageName: "src/core", Name: "core"}
	assert.Equal(t, label, label.Parent())
	label2 := BuildLabel{PackageName: "src/core", Name: "_core#src"}
	assert.Equal(t, label, label2.Parent())
	label3 := BuildLabel{PackageName: "src/core", Name: "_core_something"}
	assert.Equal(t, label3, label3.Parent())
}

func TestUnmarshalFlag(t *testing.T) {
	var label BuildLabel
	assert.NoError(t, label.UnmarshalFlag("//src/core:core"))
	assert.Equal(t, label.PackageName, "src/core")
	assert.Equal(t, label.Name, "core")
	// N.B. we can't test a failure here because it does a log.Fatalf
}

func TestUnmarshalText(t *testing.T) {
	var label BuildLabel
	assert.NoError(t, label.UnmarshalText([]byte("//src/core:core")))
	assert.Equal(t, label.PackageName, "src/core")
	assert.Equal(t, label.Name, "core")
	assert.Error(t, label.UnmarshalText([]byte(":blahblah:")))
}

func TestSubrepo(t *testing.T) {
	label := BuildLabel{Subrepo: "third_party/cc/com_google_googletest"}
	assert.Equal(t, BuildLabel{PackageName: "third_party/cc", Name: "com_google_googletest"}, label.SubrepoLabel())
}

func TestSubrepoEmpty(t *testing.T) {
	label := BuildLabel{}
	assert.Equal(t, BuildLabel{}, label.SubrepoLabel())
}

func TestParseSubrepoBuildLabel(t *testing.T) {
	label := ParseSubrepoBuildLabel("//src/core:core", "", nil)
	assert.Equal(t, "", label.Subrepo)
	subrepo := &Subrepo{Name: "test"}
	label = ParseSubrepoBuildLabel("//src/core:core", "", subrepo)
	assert.Equal(t, "test", label.Subrepo)
}

func TestTryParseSubrepoBuildLabel(t *testing.T) {
	label, err := TryParseSubrepoBuildLabel("//src/core:core", "", nil)
	assert.NoError(t, err)
	assert.Equal(t, "", label.Subrepo)
	subrepo := &Subrepo{Name: "test"}
	label, err = TryParseSubrepoBuildLabel("//src/core:core", "", subrepo)
	assert.NoError(t, err)
	assert.Equal(t, "test", label.Subrepo)
}

func TestPackageDir(t *testing.T) {
	label := NewBuildLabel("src/core", "core")
	assert.Equal(t, "src/core", label.PackageDir())
	label = NewBuildLabel("", "core")
	assert.Equal(t, ".", label.PackageDir())
}

func TestComplete(t *testing.T) {
	label := BuildLabel{}
	completions := label.Complete("//src/c")
	assert.Equal(t, 4, len(completions))
	assert.Equal(t, "//src/cache", completions[0].Item)
	assert.Equal(t, "//src/clean", completions[1].Item)
	assert.Equal(t, "//src/cli", completions[2].Item)
	assert.Equal(t, "//src/core", completions[3].Item)
}

func TestCompleteError(t *testing.T) {
	label := BuildLabel{}
	completions := label.Complete("nope")
	assert.Equal(t, 0, len(completions))
}

func TestString(t *testing.T) {
	label := BuildLabel{PackageName: "src/core", Name: "core", Subrepo: "test"}
	assert.Equal(t, "@test//src/core:core", label.String())
}

func TestFullPackageName(t *testing.T) {
	label := BuildLabel{PackageName: "src/core", Name: "core"}
	assert.Equal(t, "src/core", label.FullPackageName())
	label.Subrepo = "test"
	assert.Equal(t, "test/src/core", label.FullPackageName())
	label.PackageName = ""
	assert.Equal(t, "test", label.FullPackageName())
}

func TestMain(m *testing.M) {
	// Used to support TestComplete, the function it's testing re-execs
	// itself thinking that it's actually plz.
	if complete := os.Getenv("PLZ_COMPLETE"); complete == "//src/c" {
		os.Stdout.Write([]byte("//src/cache\n"))
		os.Stdout.Write([]byte("//src/clean\n"))
		os.Stdout.Write([]byte("//src/cli\n"))
		os.Stdout.Write([]byte("//src/core\n"))
		os.Exit(0)
	} else if complete != "" {
		os.Stderr.Write([]byte("Invalid completion\n"))
		os.Exit(1)
	}
	os.Exit(m.Run())
}
