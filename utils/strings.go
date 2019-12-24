package utils

import (
	"strings"
)

// SplitNameVersion accepts a string and returns its name and version.
func SplitNameVersion(s string) (name string, version string) {
	nameBranch := strings.Split(s, "@")
	name = nameBranch[0]
	if len(nameBranch) > 1 {
		version = nameBranch[1]
	} else {
		version = "master"
	}

	return
}
