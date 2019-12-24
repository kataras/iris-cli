package utils

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ModulePath returns the module declaration of a go.mod file "b" contents.
func ModulePath(b []byte) []byte {
	return parseDeclaration(b, moduleBytes)
}

// Package returns the package declaration (without "package") of "b" source-code contents.
func Package(b []byte) []byte {
	return parseDeclaration(b, pkgBytes)
}

var (
	slashSlash  = []byte("//")
	moduleBytes = []byte("module")
	pkgBytes    = []byte("package")
)

// parseDeclaration returns the "delcarion $TEXT" of "b" contents.
func parseDeclaration(b []byte, declaration []byte) []byte {
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

// TryFindPackage returns a go package based on the dir,
// it reads the package declaration of the `main.go` or any `*go`
func TryFindPackage(dir string) (pkg []byte) {
	if Ext(dir) != "" { // could use os.Stat but let's use just extension to decide if it's file because the "dir" may not exist yet.
		dir = filepath.Dir(dir)
	}

	d, err := os.Open(dir)
	if err != nil {
		return
	}
	files, err := d.Readdir(-1)
	d.Close()
	if err != nil {
		return
	}

	for _, f := range files {
		if f.IsDir() { // read from the first level of directory only.
			continue
		}

		if !strings.HasSuffix(f.Name(), ".go") { // read only go files.
			continue
		}

		fpath := filepath.Join(dir, f.Name())
		b, err := ioutil.ReadFile(fpath)
		if err == nil {
			if pkg = Package(b); len(pkg) > 0 {
				break
			}
		}
	}

	return
}
