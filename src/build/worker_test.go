package build

import (
	"os"
	"syscall"
	"testing"

	"github.com/kardianos/osext"
	"github.com/stretchr/testify/assert"

	"core"
)

func TestBuildWorker(t *testing.T) {
	defer func() {
		StopWorkers()
		// Shouldn't have any workers left now
		assert.Equal(t, 0, len(workerMap))
	}()
	state := core.NewBuildState(5, nil, 5, core.DefaultConfiguration())
	target := core.NewBuildTarget(core.ParseBuildLabel("//src/build:worker_test", ""))
	out, err := buildRemotely(state, "src/build/test_worker", "", "", target, nil)
	assert.NoError(t, err)
	assert.EqualValues(t, []byte{}, out)

	target.Labels = append(target.Labels, "cobol")
	out, err = buildRemotely(state, "src/build/test_worker", "", "", target, nil)
	assert.Error(t, err)
	assert.EqualValues(t, "Error building target //src/build:worker_test: COBOL is not supported, you must be joking", err.Error())

	cmd, err := osext.Executable()
	assert.NoError(t, err)

	// Fork to subprocess and check we can still use it.
	pid, err := syscall.ForkExec(cmd, []string{cmd, "--fork"}, nil)
	assert.NoError(t, err)
	subprocess, _ := os.FindProcess(pid)
	_, err = subprocess.Wait()
	assert.NoError(t, err)
}

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "--fork" {
		// We've forked from TestBuildWorker above. Build a single target.
		state := core.NewBuildState(5, nil, 5, core.DefaultConfiguration())
		target := core.NewBuildTarget(core.ParseBuildLabel("//src/build:worker_test", ""))
		_, err := buildRemotely(state, "src/build/test_worker", "", "", target, nil)
		if err != nil {
			panic(err)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
