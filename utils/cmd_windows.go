// +build windows,!appengine

package utils

import (
	"os/exec"
	"syscall"
)

// Command returns the Cmd struct to execute the named program with
// the given arguments for windows.
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}
