// +build !windows

package utils

import (
	"context"
	"io"
	"os/exec"
	"path"
	"strings"
	"syscall"

	"github.com/creack/pty"
)

// Command returns the Cmd struct to execute the named program with
// the given arguments for windows.
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// CommandWithCancel same as `Command` but returns a canceletion function too.
func CommandWithCancel(name string, args ...string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancelFunc := context.WithCancel(context.TODO())
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd, cancelFunc
}

func KillCommand(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

func FormatExecutable(bin string) string { return bin }

func StartExecutable(dir, bin string, stdout, stderr io.Writer) (*exec.Cmd, error) {
	if IsInsideDocker() {
		// If run through docker, this part is required,
		// otherwise we should NOT try this because it always gives error:
		cmd := Command("/bin/sh", "-c", bin)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // set parent group id in order to be kill-able.
		cmd.Dir = dir
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		_, err := pty.Start(cmd)
		// fork/exec /bin/sh: operation not permitted, even without setpgid.

		if err != nil {
			if !strings.Contains(err.Error(), "operation not permitted") {
				return nil, err
			}
		}
	}

	cmd := Command(path.Join(dir, bin))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return cmd, nil
}
