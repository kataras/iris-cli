// +build windows,!appengine

package utils

import (
	"io"
	"os/exec"
	"strconv"
	"strings"
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
	kill := exec.Command("TASKKILL", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	return kill.Run()
}

func FormatExecutable(bin string) string {
	if ext := ".exe"; !strings.HasPrefix(bin, ext) {
		bin += ext
	}

	return bin
}

func StartExecutable(dir, bin string, stdout, stderr io.Writer) (*exec.Cmd, error) {
	cmd := Command("cmd", "/c", bin)
	// cmd := Command(bin) // here the cmd.Process.Pid will give the program's correct PID
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd, cmd.Start()
}
