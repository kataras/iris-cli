package utils

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	FileRename = fsnotify.Rename
)

type Filter func(name string) bool

type Watcher struct {
	watcher  *fsnotify.Watcher
	rootDirs []string

	AddFilter Filter

	Events chan []fsnotify.Event
	closed chan struct{}
}

func (w *Watcher) AddRecursively(root string) error {
	dirs, err := FindMatches(root, "*", "*\\node_modules", true) // including "root" itself.
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		if err = w.Add(dir); err != nil {
			return err
		}
	}

	w.rootDirs = append(w.rootDirs, root)

	return nil
}

func (w *Watcher) Add(dir string) error {
	if filter := w.AddFilter; filter != nil {
		if !filter(dir) {
			return nil
		}
	}

	return w.watcher.Add(dir)
}

func (w *Watcher) Is(evt fsnotify.Event, op fsnotify.Op) bool {
	return evt.Op&op == op
}

func (w *Watcher) Close() error {
	close(w.closed)
	return w.watcher.Close()
}

func (w *Watcher) Closed() <-chan struct{} {
	return w.closed
}

func (w *Watcher) start() {
	ticker := time.NewTicker(1 * time.Second)
	evts := make([]fsnotify.Event, 0)

	for {
		select {
		case event := <-w.watcher.Events:
			if event.Name == "" {
				continue
			}

			if event.Op&fsnotify.Chmod == fsnotify.Chmod {
				continue
			}

			evts = append(evts, event)
		case <-ticker.C:
			if len(evts) > 0 {
				w.Events <- evts
				evts = evts[0:0]
			}
		case <-w.closed:
			ticker.Stop()
			if len(evts) > 0 { // flush events.
				w.Events <- evts
			}
			// close(w.Events)
			return
		}
	}
}

func NewWatcher() (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher: fsWatcher,
		Events:  make(chan []fsnotify.Event, 1),
		closed:  make(chan struct{}, 1),
	}

	go w.start()
	return w, nil
}
