// +build windows,!appengine

package utils

import (
	"os/exec"
	"strconv"
	"syscall"
)

// Command returns the Cmd struct to execute the named program with
// the given arguments for windows.
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func KillCommand(cmd *exec.Cmd) error {
	pid := cmd.Process.Pid
	kill := exec.Command("TASKKILL", "/T", "/F", "/PID", strconv.Itoa(pid))
	return kill.Run()
}
