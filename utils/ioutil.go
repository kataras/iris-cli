package utils

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	mime.AddExtensionType(".go", "text/plain; charset=utf-8")
	mime.AddExtensionType(".mod", "text/plain; charset=utf-8")
	mime.AddExtensionType(".sum", "text/plain; charset=utf-8")
	mime.AddExtensionType(".json", "application/json; charset=utf-8")
	mime.AddExtensionType(".yaml", "application/yaml; charset=utf-8")
	mime.AddExtensionType(".yml", "application/yaml; charset=utf-8")
	mime.AddExtensionType(".toml", "application/toml; charset=utf-8")
	mime.AddExtensionType(".tml", "application/tml; charset=utf-8")
}

// NoOpReadCloser wraps the "r" and returns a new io.ReadCloser which its `Close` does nothing.
func NoOpReadCloser(r io.Reader) io.ReadCloser {
	return noOpCloser{r}
}

type noOpCloser struct {
	io.Reader
}

func (r noOpCloser) Close() error { return nil }

type multiCloser struct {
	io.Reader
	closers []io.ReadCloser
}

func (r multiCloser) Close() (err error) {
	for _, c := range r.closers {
		if cErr := c.Close(); cErr != nil {
			if err == nil {
				err = cErr
			} else {
				err = fmt.Errorf("%w\n%w", err, cErr)
			}
		}
	}

	return
}

// Exists tries to report whether the local physical "path" exists.
func Exists(path string) bool {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return false
	}

	// It exists but it can cause other errors when reading but we don't care here.
	return true
}

// Ext returns the filepath extension of "s".
func Ext(s string) string {
	if idx := strings.LastIndexByte(s, '.'); idx > 0 && len(s)-1 > idx {
		ext := s[idx:] // including dot.
		if mime.TypeByExtension(ext) != "" {
			// to avoid act paths like "$gopath/src/github.com/project" as file.
			return ext
		}
	}

	return ""
}

// Dest tries to resolve and returns a destination dir path.
func Dest(dest string) string {
	if dest == "" {
		dest, _ = os.Getwd()
	} else if s := "%GOPATH%"; strings.Contains(dest, s) {
		gopath := os.Getenv("GOPATH")
		if gopath != "" {
			gopath = filepath.Join(gopath, "src")
			dest = strings.Replace(dest, s, gopath, 1)
		}
	}

	d, err := filepath.Abs(dest)
	if err == nil {
		dest = d
	}

	return filepath.Clean(dest)
}
