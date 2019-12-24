package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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

// Ext returns the filepath extension of "s"
func Ext(s string) string {
	if idx := strings.LastIndexByte(s, '.'); idx > 0 && len(s)-1 > idx {
		return s[idx:] // including dot.
	}

	return ""
}

// Dest tries to resolve and returns a destination dir path.
func Dest(dest string) string {
	if dest == "" {
		dest, _ = os.Getwd()
	} else if s := "%GOPATH"; strings.Contains(dest, s) {
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

// ModulePath returns the module declaration of a go.mod file "b" contents.
func ModulePath(b []byte) []byte {
	return findDeclaration(b, moduleBytes)
}

// Package returns the package declaration (without "package") of "b" source-code contents.
func Package(b []byte) []byte {
	return findDeclaration(b, pkgBytes)
}

var (
	slashSlash  = []byte("//")
	moduleBytes = []byte("module")
	pkgBytes    = []byte("package")
)

// findDeclaration returns the "delcarion $TEXT" of "b" contents.
func findDeclaration(b []byte, declaration []byte) []byte {
	for len(b) > 0 {
		line := b
		b = nil
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			line, b = line[:i], line[i+1:]
		}
		if i := bytes.Index(line, slashSlash); i >= 0 {
			line = line[:i]
		}
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, declaration) {
			continue
		}
		line = line[len(declaration):]
		n := len(line)
		line = bytes.TrimSpace(line)
		if len(line) == n || len(line) == 0 {
			continue
		}

		if line[0] == '"' || line[0] == '`' {
			p, err := strconv.Unquote(string(line))
			if err != nil {
				return nil // malformed quoted string or multiline string
			}
			return []byte(p)
		}

		return line
	}

	return nil
}
