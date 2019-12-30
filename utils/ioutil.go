package utils

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
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

// IsDir reports whether a "path" is a filesystem directory.
func IsDir(path string) bool {
	if f, err := os.Stat(path); err == nil {
		return f.IsDir()
	}

	return false
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

// FindMatches find all matches of a "pattern" reclusively.
// Ordered by parent dir.
func FindMatches(rootDir, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			return err
		}

		if matched {
			matches = append(matches, path)
		}

		return nil
	})

	sort.Slice(matches, func(i, j int) bool {
		ni := strings.Count(matches[i], string(os.PathSeparator))
		nj := strings.Count(matches[j], string(os.PathSeparator))
		return ni < nj
	})

	return matches, err
}
