package core

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// safeBuffer is an io.Writer that ensures that only one thread writes to it at a time.
// This is important because we potentially have both stdout and stderr writing to the same
// buffer, and os.exec only guarantees goroutine-safety if both are the same writer, which in
// our case they're not (but are both ultimately causing writes to the same buffer)
type safeBuffer struct {
	sync.Mutex
	buf bytes.Buffer
}

func (sb *safeBuffer) Write(b []byte) (int, error) {
	sb.Lock()
	defer sb.Unlock()
	return sb.buf.Write(b)
}

func (sb *safeBuffer) Bytes() []byte {
	return sb.buf.Bytes()
}

func (sb *safeBuffer) String() string {
	return sb.buf.String()
}

// logProgress logs a message once a minute until the given context has expired.
// Used to provide some notion of progress while waiting for external commands.
func logProgress(label BuildLabel, ctx context.Context) {
	t := time.NewTicker(1 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if i == 1 {
				log.Notice("%s still running after 1 minute", label)
			} else {
				log.Notice("%s still running after %d minutes", label, i)
			}
		}
	}
}

// ExecWithTimeout runs an external command with a timeout.
// If the command times out the returned error will be a context.DeadlineExceeded error.
// If showOutput is true then output will be printed to stderr as well as returned.
// It returns the stdout only, combined stdout and stderr and any error that occurred.
func ExecWithTimeout(target *BuildTarget, dir string, env []string, timeout time.Duration, defaultTimeout cli.Duration, showOutput bool, argv []string, sandbox bool) ([]byte, []byte, error) {
	// Runtime check is a little ugly, but we know this only works on Linux right now.
	if sandbox && runtime.GOOS == "linux" {
		tool, err := LookPath(State.Config.Build.PleaseSandboxTool, State.Config.Build.Path)
		if err != nil {
			return nil, nil, err
		}
		c = append([]string{tool}, c...)
	}
	if timeout == 0 {
		if defaultTimeout == 0 {
			timeout = 10 * time.Minute
		} else {
			timeout = time.Duration(defaultTimeout)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.Command(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = env

	var out bytes.Buffer
	var outerr safeBuffer
	if showOutput {
		cmd.Stdout = io.MultiWriter(os.Stderr, &out, &outerr)
		cmd.Stderr = io.MultiWriter(os.Stderr, &outerr)
	} else {
		cmd.Stdout = io.MultiWriter(&out, &outerr)
		cmd.Stderr = &outerr
	}
	if target != nil {
		go logProgress(target.Label, ctx)
	}
	// Start the command, wait for the timeout & then kill it.
	// We deliberately don't use CommandContext because it will only send SIGKILL which
	// child processes can't handle themselves.
	err := cmd.Start()
	if err != nil {
		return nil, nil, err
	}
	ch := make(chan error)
	go runCommand(cmd, ch)
	select {
	case err = <-ch:
		// Do nothing.
	case <-time.After(timeout):
		// Send a relatively gentle signal that it can catch.
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			log.Notice("Failed to kill subprocess: %s", err)
		}
		time.Sleep(10 * time.Millisecond)
		// Send a more forceful signal.
		cmd.Process.Kill()
		err = fmt.Errorf("Timeout exceeded: %s", outerr.String())
	}
	return out.Bytes(), outerr.Bytes(), err
}

// runCommand runs a command and signals on the given channel when it's done.
func runCommand(cmd *exec.Cmd, ch chan error) {
	ch <- cmd.Wait()
}

// ExecWithTimeoutShell runs an external command within a Bash shell.
// Other arguments are as ExecWithTimeout.
// Note that the command is deliberately a single string.
func ExecWithTimeoutShell(target *BuildTarget, dir string, env []string, timeout time.Duration, defaultTimeout cli.Duration, showOutput bool, cmd string, sandbox bool) ([]byte, []byte, error) {
	c := append([]string{"bash", "-u", "-o", "pipefail", "-c"}, cmd)
	return ExecWithTimeout(target, dir, env, timeout, defaultTimeout, showOutput, c, sandbox)
}

// ExecWithTimeoutSimple runs an external command with a timeout.
// It's a simpler version of ExecWithTimeout that gives less control.
func ExecWithTimeoutSimple(timeout cli.Duration, cmd ...string) ([]byte, error) {
	_, out, err := ExecWithTimeout(nil, "", nil, time.Duration(timeout), timeout, false, cmd)
	return out, err
}

// sandboxer manages a limited collection of sandbox subprocesses for us.
// This is essentially a performance hack since CLONE_NEWNET is surprisingly expensive
// (typically adding 3-400ms overhead, which is more than we can tolerate).
// It seems possible that this is fixed in Linux 4.12 (?) so we could revisit later, but it
// would have to be a long way down the line.
type sandboxer struct {
	sync.Mutex
	allSlaves []*sandboxSlave
	available []*sandboxSlave
}

// A sandboxSlave represents one of our slave processes.
type sandboxSlave struct {
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	stdout, stderr io.ReadCloser
}

// Submit runs a new task within one of the sandboxed processes.
func (s *sandboxer) Submit(dir string, env []string, timeout time.Duration, argv []string) ([]byte, []byte, error) {
	slave := s.getSlave()
	slave.stdin.Write([]byte(fmt.Sprintf("%s\x1d%s\x1d%s\x1c", dir, strings.Join(env, "\x1e"), strings.Join(argv, "\x1e"))))
}

func (s *sandboxer) getSlave() (*sandboxSlave, error) {
	s.Lock()
	defer s.Unlock()
	if len(s.available) > 0 {
		// Reuse existing slave process.
		slave := s.available[len(s.available)-1]
		s.available = s.available[:len(s.available)-1]
		return slave, nil
	}
	// Need to create a new one.
	tool, err := LookPath(State.Config.Build.PleaseSandboxTool, State.Config.Build.Path)
	if err != nil {
		return nil, error
	}
	cmd := exec.Command(tool)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, error
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, error
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, error
	}
	if err := cmd.Start(); err != nil {
		return nil, error
	}
	slave := &sandboxSlave{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}
	s.allSlaves = append(s.allSlaves, slave)
	return slave, nil
}

func (s *sandboxer) releaseSlave(slave *sandboxSlave) {
	s.Lock()
	defer s.Unlock()
	s.available = append(s.available, slave)
}
