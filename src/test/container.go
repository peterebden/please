//+build !bootstrap

// Support for containerising tests. Currently Docker only.

package test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"docker.io/go-docker"
	"docker.io/go-docker/api"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"docker.io/go-docker/api/types/mount"

	"build"
	"core"
)

var dockerClient *docker.Client
var dockerClientOnce sync.Once

func runContainerisedTest(state *core.BuildState, target *core.BuildTarget) (out []byte, err error) {
	const resultsFile = "/tmp/test.results"
	const testDir = "/tmp/test"

	dockerClientOnce.Do(func() {
		httpClient := &http.Client{Timeout: time.Second * 20}
		dockerClient, err = docker.NewClient(docker.DefaultDockerHost, api.DefaultVersion, httpClient, nil)
	})
	if err != nil {
		return nil, err
	} else if dockerClient == nil {
		return nil, fmt.Errorf("failed to initialise client")
	}

	targetTestDir := path.Join(core.RepoRoot, target.TestDir())
	replacedCmd := build.ReplaceTestSequences(state, target, target.GetTestCommand(state))
	replacedCmd += " " + strings.Join(state.TestArgs, " ")
	// Gentle hack: remove the absolute path from the command
	replacedCmd = strings.Replace(replacedCmd, targetTestDir, targetTestDir, -1)

	timeout := int(target.TestTimeout.Seconds())
	if timeout == 0 {
		timeout = int(time.Duration(state.Config.Test.Timeout).Seconds())
	}

	env := core.BuildEnvironment(state, target, true)
	env.Replace("RESULTS_FILE", resultsFile)
	env.Replace("GTEST_OUTPUT", "xml:"+resultsFile)

	config := &container.Config{
		Image: state.Config.Docker.DefaultImage,
		// TODO(peterebden): Do we still need LC_ALL here? It was kinda hacky before...
		Env:         append(env, "LC_ALL=C.UTF-8"),
		WorkingDir:  testDir,
		Cmd:         []string{"bash", "-c", replacedCmd},
		StopTimeout: &timeout,
		Tty:         true, // This makes it a lot easier to read later on.
	}
	if target.ContainerSettings != nil {
		if target.ContainerSettings.DockerImage != "" {
			config.Image = target.ContainerSettings.DockerImage
		}
		config.User = target.ContainerSettings.DockerUser
	}
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{{
			Type:   mount.TypeBind,
			Source: testDir,
			Target: config.WorkingDir,
		}},
	}
	log.Debug("Running %s in container. Equivalent command: docker run -it --rm -e %s -u \"%s\" %s %s",
		strings.Join(config.Env, " -e "), config.User, config.Image, strings.Join(config.Cmd, " "))
	c, err := dockerClient.ContainerCreate(context.Background(), config, hostConfig, nil, "")
	if err != nil {
		return nil, fmt.Errorf("Failed to create container: %s", err)
	}
	for _, warning := range c.Warnings {
		log.Warning("%s creating container: %s", target.Label, warning)
	}
	defer func() {
		if err := dockerClient.ContainerRemove(context.Background(), c.ID, types.ContainerRemoveOptions{
			RemoveVolumes: true,
			RemoveLinks:   true,
			Force:         true,
		}); err != nil {
			log.Warning("Failed to remove container for %s: %s", target.Label, err)
		}
	}()
	if err := dockerClient.ContainerStart(context.Background(), c.ID, types.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("Failed to start container: %s", err)
	}
	waitChan, errChan := dockerClient.ContainerWait(context.Background(), c.ID, container.WaitConditionNotRunning)
	var status int64
	select {
	case body := <-waitChan:
		status = body.StatusCode
	case err := <-errChan:
		return nil, fmt.Errorf("Container failed running: %s", err)
	}
	// Now retrieve the results and any other files.
	if !target.NoTestOutput {
		retrieveFile(state, target, c.ID, resultsFile, true)
	}
	if state.NeedCoverage {
		retrieveFile(state, target, c.ID, path.Join(testDir, "test.coverage"), false)
	}
	for _, output := range target.TestOutputs {
		retrieveFile(state, target, c.ID, path.Join(testDir, output), false)
	}
	r, err := dockerClient.ContainerLogs(context.Background(), c.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving container output: %s", err)
	} else if status != 0 {
		return b, fmt.Errorf("Exit code %d", status)
	}
	return b, nil
}

func runPossiblyContainerisedTest(state *core.BuildState, target *core.BuildTarget) (out []byte, err error) {
	if target.Containerise {
		if state.Config.Test.DefaultContainer == core.ContainerImplementationNone {
			log.Warning("Target %s specifies that it should be tested in a container, but test "+
				"containers are disabled in your .plzconfig.", target.Label)
			return runTest(state, target)
		}
		out, err = runContainerisedTest(state, target)
		if err != nil && state.Config.Docker.AllowLocalFallback {
			log.Warning("Failed to run %s containerised: %s %s. Falling back to local version.",
				target.Label, out, err)
			return runTest(state, target)
		}
		return out, err
	}
	return runTest(state, target)
}

// retrieveFile retrieves a single file (or directory) from a Docker container.
func retrieveFile(state *core.BuildState, target *core.BuildTarget, cid string, filename string, warn bool) {
	if err := retrieveOneFile(state, target, cid, filename); err != nil {
		if warn {
			log.Warning("Failed to retrieve results for %s: %s", target.Label, err)
		} else {
			log.Debug("Failed to retrieve results for %s: %s", target.Label, err)
		}
	}
}

// retrieveOneFile retrieves a single file from a Docker container.
func retrieveOneFile(state *core.BuildState, target *core.BuildTarget, cid string, filename string) error {
	log.Debug("Attempting to retrieve file %s for %s...", filename, target.Label)
	r, _, err := dockerClient.CopyFromContainer(context.Background(), cid, filename)
	if err != nil {
		return err
	}
	// TODO(peterebden): Handle this being a directory (not sure how we can read it?)
	defer r.Close()
	f, err := os.Create(path.Join(target.TestDir(), path.Base(filename)))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}
