//go:build windows && !appengine
// +build windows,!appengine

package utils

import (
	"context"
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

// CommandWithCancel same as `Command` but returns a canceletion function too.
func CommandWithCancel(name string, args ...string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd, func() {
		if cmd != nil {
			if cmd.ProcessState == nil { // it's not already closed.
				if cmd.Process != nil && cmd.Process.Pid > 0 {
					// println("Killing: " + name + strings.Join(args, " "))
					_ = KillCommand(cmd)
				}
			}

			cancelFunc()
		}
	}
}

func KillCommand(cmd *exec.Cmd) error {
	kill := exec.Command("TASKKILL", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	return kill.Run()
}

func FormatExecutable(bin string) string {
	if ext := ".exe"; !strings.HasSuffix(bin, ext) {
		bin += ext
	}

	return bin
}

func StartExecutable(dir, bin string, stdout, stderr io.Writer) (*exec.Cmd, error) {
	cmd := Command("cmd", "/c", bin)
	// cmd, cancelFunc := CommandWithCancel(bin) // here the cmd.Process.Pid will give the program's correct PID
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd, cmd.Start()
}
