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
	Repo    string `json:"repo" yaml:"Repo" toml:"Repo"`                    // e.g. "iris-contrib/starter-kit"
	Version string `json:"version,omitempty" yaml:"Version" toml:"Version"` // if empty then set to "master"
	// Local.
	Dest   string `json:"dest,omitempty" yaml:"Dest" toml:"Dest"`       // if empty then $GOPATH+Module or ./+Module
	Module string `json:"module,omitempty" yaml:"Module" toml:"Module"` // if empty then set to the remote module name fetched from go.mod

	// Pre Installation.
	Reader func(io.Reader) ([]byte, error) `json:"-" yaml:"-" toml:"-"`
	// Post Installation.
	// InstalledPath string `json:"-" yaml:"-" toml:"-"` // the dest + name filepath if installed, if empty then it is not installed yet.
}

func New(name, repo string) *Project {
	name, version := utils.SplitNameVersion(name) // i.e. github.com/author/project@v12
	if version == "" {
		repo, version = utils.SplitNameVersion(repo) // check the repo suffix too.
		if version == "" {
			version = "master"
		}
	}

	return &Project{
		Name:    name,
		Repo:    repo,
		Version: version,
		Dest:    "",
		Module:  "",
	}
}

func Run(projectPath string, stdOut, stdErr io.Writer) error {
	goRun := utils.Command("go", "run", ".")
	goRun.Dir = projectPath
	goRun.Stdout = stdOut
	goRun.Stderr = stdErr
	return goRun.Run()
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

	p.Repo = strings.TrimLeft(p.Repo, "https://github.com/")

	zipURL := fmt.Sprintf("https://github.com/%s/archive/%s.zip", p.Repo, p.Version) // e.g. https://github.com/kataras/iris-cli/archive/master.zip
	r, err := utils.DownloadReader(zipURL, nil)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if p.Reader != nil {
		return p.Reader(r)
	}

	return ioutil.ReadAll(r)
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

			oldModuleName = utils.ModulePath(contents)
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

	p.Dest = utils.Dest(p.Dest)

	for _, f := range r.File {
		// without the /$project-$version root folder, so it can be used to dest as it is without creating a new folder based on the project name.
		name := strings.TrimPrefix(f.Name, compressedRootFolder)
		if name == "" {
			// root folder.
			continue
		}
		fpath := filepath.Join(p.Dest, name)

		// https://snyk.io/research/zip-slip-vulnerability#go
		if !strings.HasPrefix(fpath, p.Dest+string(os.PathSeparator)) {
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

		rc, err := f.Open()
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
			_, err = outFile.Write(newContents)
		} else {
			_, err = io.Copy(outFile, rc)
		}

		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	// Don't use Module name for path because it may contains a version suffix.
	// newPath := filepath.Join(dest, p.Name)
	// os.RemoveAll(newPath)
	// err = os.Rename(filepath.Join(dest, compressedRootFolder), newPath)
	// if err == nil {
	// 	p.InstalledPath = newPath
	// }

	return nil
}
