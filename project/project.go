package project

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/kataras/iris-cli/utils"
)

type Project struct {
	Name string `json:"name,omitempty" yaml:"Name" toml:"Name"` // e.g. starter-kit
	// Remote.
	Repo    string `json:"repo" yaml:"Repo" toml:"Repo"`                    // e.g. "iris-contrib/project1"
	Version string `json:"version,omitempty" yaml:"Version" toml:"Version"` // if empty then set to "master"
	// Local.
	Dest   string `json:"dest,omitempty" yaml:"Dest" toml:"Dest"`       // if empty then $GOPATH+Module or ./+Module
	Module string `json:"module,omitempty" yaml:"Module" toml:"Module"` // if empty then set to the remote module name fetched from go.mod
}

func SplitName(s string) (name string, version string) {
	nameBranch := strings.Split(s, "@")
	name = nameBranch[0]
	if len(nameBranch) > 1 {
		version = nameBranch[1]
	} else {
		version = "master"
	}

	return
}

func New(name, repo string) *Project {
	name, version := SplitName(name) // i.e. github.com/author/project@v12
	if version == "" {
		repo, version = SplitName(repo) // check the repo suffix too.
	}
	return &Project{
		Name:    name,
		Repo:    repo,
		Version: version,
		Dest:    "",
		Module:  "",
	}
}

func (p *Project) Install() error {
	b, err := p.download()
	if err != nil {
		return err
	}

	return p.unzip(b)
}

func (p *Project) download() ([]byte, error) {
	p.Version = strings.Split(p.Version, " ")[0]
	if p.Version == "latest" {
		p.Version = "master"
	}

	zipURL := fmt.Sprintf("https://github.com/%s/archive/%s.zip", p.Repo, p.Version) // e.g. https://github.com/kataras/iris-cli/archive/master.zip
	return utils.Download(zipURL, nil)
}

func (p *Project) unzip(body []byte) error {
	r, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return err
	}

	if len(r.File) == 0 {
		return fmt.Errorf("empty zip")
	}

	first := r.File[0]
	if !first.FileInfo().IsDir() {
		return fmt.Errorf("expected a root folder but got <%s>", first.Name)
	}

	if base := filepath.Base(p.Repo); !strings.Contains(first.Name, base) {
		return fmt.Errorf("expected root folder to match the repository name <%s> but got <%s>", base, first.Name)
	}
	compressedRootFolder := first.Name // e.g. iris-master

	var oldModuleName []byte
	// Find current module name, starting from the end because list is sorted alphabetically
	// and "go.mod" is more likely to be visible at the end.
	modFile := filepath.Join(compressedRootFolder, "go.mod")
	for i := len(r.File) - 1; i > 0; i-- {
		f := r.File[i]
		if filepath.Clean(f.Name) == modFile {
			rc, err := f.Open()
			if err != nil {
				return err
			}

			contents, err := ioutil.ReadAll(rc)
			if err != nil {
				return err
			}

			oldModuleName = []byte(utils.ModulePath(contents))
			if p.Module == "" {
				// if new module name is empty, then default it to the remote one.
				p.Module = string(oldModuleName)
			}

			break
		}
	}

	if len(oldModuleName) == 0 {
		// no go mod found, stop here  as we dont' support non-go modules, Iris depends on go 1.13.
		return fmt.Errorf("project <%s> version <%s> is not a go module, please try other version", p.Name, p.Version)
	}

	var (
		newModuleName = []byte(p.Module)
		shouldReplace = !bytes.Equal(oldModuleName, newModuleName)
	)

	// If destination is empty then set it to %GOPATH%+newModuleName.
	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		gopath = filepath.Join(gopath, "src")
	}

	dest := p.Dest
	if dest == "" {
		if gopath != "" {
			dest = filepath.Join(gopath, filepath.Dir(p.Module))
		} else {
			dest, _ = os.Getwd()
		}
	} else {
		dest = strings.Replace(dest, "%GOPATH%", gopath, 1)
		d, err := filepath.Abs(dest)
		if err == nil {
			dest = d
		}
	}
	p.Dest = dest

	for _, f := range r.File {
		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)

		// https://snyk.io/research/zip-slip-vulnerability#go
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		var rc io.ReadCloser

		rc, err = f.Open()
		if err != nil {
			return err
		}

		// If new(local) module name differs the current(remote) one.
		if shouldReplace {
			contents, err := ioutil.ReadAll(rc)
			if err != nil {
				return err
			}

			newContents := bytes.ReplaceAll(contents, oldModuleName, newModuleName)
			rc = utils.NoOpReadCloser(bytes.NewReader(newContents))
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	// Don't use Module name for path because it may contains a version suffix.
	newPath := filepath.Join(dest, p.Name)
	os.RemoveAll(newPath)
	return os.Rename(filepath.Join(dest, compressedRootFolder), newPath)
	return nil
}
