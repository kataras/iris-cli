// +build !windows

package utils

import (
	"os/exec"
)

// Command returns the Cmd struct to execute the named program with
// the given arguments.
func Command(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
