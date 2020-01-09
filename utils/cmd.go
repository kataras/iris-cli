// +build !windows

package utils

import (
	"io"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// Command returns the Cmd struct to execute the named program with
// the given arguments.
func Command(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func KillCommand(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

func FormatExecutable(bin string) string { return bin }

func StartExecutable(dir, bin string, stdout, stderr io.Writer) (*exec.Cmd, error) {
	cmd := Command("/bin/sh", "-c", bin)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // set parent group id in order to be kill-able.
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	f, err := pty.Start(c) // it runs cmd.Start().
	if err != nil {
		return nil, err
	}

	return cmd, err
}
