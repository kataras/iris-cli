package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
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
				err = fmt.Errorf("%v\n%v", err, cErr)
			}
		}
	}

	return
}

// Exists tries to report whether the local physical "path" exists.
func Exists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return os.IsExist(err) // It exists but it can cause other errors when reading but we don't care here.
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

var defaultExcludePatterns = []string{
	"*\\node_modules",
	"*\\.git",
}

// FindMatches find all matches of a "pattern" reclusively.
// Ordered by parent dir.
func FindMatches(rootDir, pattern string, listDirectories bool) ([]string, error) {
	var matches []string
	matchAny := pattern == "*"
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(rootDir, path)

		for _, exclude := range defaultExcludePatterns {
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

// GetAllFiles returns all files and directories from "rootDir".
// The return "files" as fullpaths.
func GetAllFiles(rootDir string) (files []string, err error) {
	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == rootDir {
			return nil
		}

		files = append(files, path)
		return nil
	})

	return
}

// GetFilesDiff returns a function which collects
// new files or directories since `GetFilesDiff` called.
// Its return function's slice contains a relative to "rootDir" filenames.
func GetFilesDiff(rootDir string) (func() []string, error) {
	prevFiles, err := GetAllFiles(rootDir)
	if err != nil {
		return nil, err
	}

	collect := func() []string {
		var newFiles []string

		err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if path == rootDir {
				return nil
			}

			ignore := false
			for i, prev := range prevFiles {
				if path == prev {
					ignore = true
					prevFiles = append(prevFiles[:i], prevFiles[i+1:]...)
					break
				}
			}

			if !ignore {
				newFiles = append(newFiles, filepath.ToSlash(strings.TrimPrefix(path, filepath.FromSlash(rootDir)+string(os.PathSeparator))))
				if info.IsDir() { // if it's a new directory let's just add this and not its contents.
					return filepath.SkipDir
				}
			}

			return nil
		})

		if err != nil {
			return nil
		}

		return newFiles
	}

	return collect, nil
}

// IsInsideDocker reports whether the iris-cli is running through a docker container.
func IsInsideDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil || os.IsExist(err)
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

	paused *uint32

	Dirs []string
}

func (w *Watcher) AddRecursively(root string) error {
	dirs, err := FindMatches(root, "*", true) // including "root" itself.
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		dir = filepath.ToSlash(dir)
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

	w.Dirs = append(w.Dirs, dir)

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

func (w *Watcher) Pause() bool {
	return atomic.CompareAndSwapUint32(w.paused, 0, 1)
}

func (w *Watcher) Continue() bool {
	return atomic.CompareAndSwapUint32(w.paused, 1, 0)
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
				if atomic.LoadUint32(w.paused) == 0 {
					w.Events <- evts
					evts = evts[0:0]
				}
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
		paused:  new(uint32),
	}

	go w.start()
	return w, nil
}

// Export exports "v" to "destFile" system file.
// It creates it if it does not exist and overrides it if contains data.
func Export(destFile string, v interface{}) error {
	destFile = filepath.ToSlash(destFile)
	if dir := path.Dir(destFile); len(dir) > 1 {
		os.MkdirAll(dir, 0666)
	}

	f, err := os.OpenFile(destFile, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	var encoder interface {
		Encode(interface{}) error
	}

	switch ext := path.Ext(destFile); ext {
	case ".json":
		encoder = json.NewEncoder(f)
	case ".yml", ".yaml":
		enc := yaml.NewEncoder(f)
		defer enc.Close()
		encoder = enc
	default:
		return fmt.Errorf("unexpected file extension: %s", ext)
	}

	return encoder.Encode(v)
}

// Import decodes a file to "dest".
func Import(sourceFile string, dest interface{}) error {
	f, err := os.Open(sourceFile)
	if err != nil {
		return err
	}
	defer f.Close()

	if st, err := f.Stat(); err != nil {
		return err
	} else if length := st.Size(); length == 0 {
		return nil // it may exists but empty.
	}

	var decoder interface {
		Decode(interface{}) error
	}

	switch ext := path.Ext(filepath.ToSlash(sourceFile)); ext {
	case ".json":
		decoder = json.NewDecoder(f)
	case ".yml", ".yaml":
		decoder = yaml.NewDecoder(f)
	default:
		return fmt.Errorf("unexpected file extension: %s", ext)
	}

	return decoder.Decode(dest)
}
