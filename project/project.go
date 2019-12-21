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
	Repo   string `json:"repo" yaml:"Repo" toml:"Repo"`                 // e.g. "github.com/iris-contrib/project1"
	Branch string `json:"branch,omitempty" yaml:"Branch" toml:"Branch"` // if empty then set to "master"
	// Local.
	Dest   string `json:"dest,omitempty" yaml:"Dest" toml:"Dest"`       // if empty then $GOPATH+Module or ./+Module
	Module string `json:"module,omitempty" yaml:"Module" toml:"Module"` // if empty then set to the remote module name fetched from go.mod
}

func New(dest, repo string) *Project {
	repoBranch := strings.Split(repo, "@") // i.e. github.com/author/project@v12
	p := &Project{
		Name:   "",
		Repo:   repoBranch[0],
		Branch: "master",
		Dest:   dest,
		Module: "",
	}
	if len(repoBranch) > 1 {
		p.Branch = repoBranch[1]
	}

	return p
}

func (p *Project) Install() error {
	b, err := p.download()
	if err != nil {
		return err
	}

	return p.unzip(b)
}

func (p *Project) download() ([]byte, error) {
	zipURL := fmt.Sprintf("%s/archive/%s.zip", p.Repo, p.Branch) // e.g. https://github.com/kataras/iris-cli/archive/master.zip
	if !strings.HasPrefix(p.Repo, "http") {
		zipURL = "https://" + zipURL
	}

	return utils.Download(zipURL, nil)
}

func (p *Project) unzip(body []byte) error {
	compressedRootFolder := filepath.Base(p.Repo) + "-" + p.Branch // e.g. iris-master
	r, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return err
	}

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

	newpath := filepath.Join(dest, filepath.Base(p.Module))
	os.RemoveAll(newpath)
	return os.Rename(filepath.Join(dest, compressedRootFolder), newpath)
}
