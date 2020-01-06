package utils

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fsnotify/fsnotify"
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
	if _, err := os.Stat(path); err != nil {
		if os.IsExist(err) {
			// It exists but it can cause other errors when reading but we don't care here.
			return true
		}

		return false
	}

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

	return filepath.ToSlash(filepath.Clean(dest))
}

// FindMatches find all matches of a "pattern" reclusively.
// Ordered by parent dir.
func FindMatches(rootDir, pattern, exclude string, listDirectories bool) ([]string, error) {
	var matches []string
	matchAny := pattern == "*"

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if exclude != "" {
			rel, _ := filepath.Rel(rootDir, path)
			if ignore, _ := filepath.Match(exclude, rel); ignore {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

		}

		if info.IsDir() {
			if !listDirectories { // it's a directory but we don't want to list them.
				return nil
			}
		} else if listDirectories { // list only directories but this is not a directory.
			return nil
		}

		matched := matchAny
		if !matchAny {
			name := path
			if !listDirectories {
				name = filepath.Base(name)
			}

			matched, err = filepath.Match(pattern, name)
			if err != nil {
				return err
			}
		}

		if matched {
			matches = append(matches, path)
		}

		return nil
	})

	if len(matches) > 1 {
		sort.Slice(matches, func(i, j int) bool {
			ni := strings.Count(matches[i], string(os.PathSeparator))
			nj := strings.Count(matches[j], string(os.PathSeparator))
			return ni < nj
		})
	}

	return matches, err
}

const (
	FileCreate = fsnotify.Create
	FileWrite  = fsnotify.Write
	FileRemove = fsnotify.Remove
)

type WatchFileEvents map[fsnotify.Op]func(filename string)

func WatchFileChanges(rootDir string, events WatchFileEvents) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	doneCh := make(chan struct{})
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				for op, cb := range events {
					if event.Op&op == op {
						cb(event.Name)
					}
				}
			case <-doneCh:
				if watcher != nil {
					watcher.Close()
				}
				return
			}
		}
	}()

	directoriesToWatch, err := FindMatches(rootDir, "*", "*\\node_modules", true) // including rootDir itself.
	if err != nil {
		close(doneCh)
		return nil, err
	}

	for _, dir := range directoriesToWatch {
		/* e.g.
		watch: C:\Users\kataras\Desktop\myproject
		watch: C:\Users\kataras\Desktop\myproject\app
		watch: C:\Users\kataras\Desktop\myproject\app\public
		watch: C:\Users\kataras\Desktop\myproject\app\src
		*/
		if err = watcher.Add(dir); err != nil {
			close(doneCh)
			return nil, err
		}
	}

	return watcher, nil
}
