package project

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	// Post installation.
	// absolute path of the files and directories installed, because the folder may be not empty
	// and when installation fails we don't want to delete any user-defined files, just the project's ones.
	Files []string
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
	var runCmd *exec.Cmd

	if utils.IsDir(projectPath) {
		runScriptExt := ".bat"
		if runtime.GOOS != "windows" {
			runScriptExt = ".sh"
		}

		if runScriptPath := filepath.Join(projectPath, "run"+runScriptExt); utils.Exists(runScriptPath) {
			// run.bat or run.sh exists
			runCmd = utils.Command(runScriptPath)
		} else {
			// else check for Makefile(make) or Makefile.win (nmake).
			makefilePath := filepath.Join(projectPath, "Makefile")
			makefileExists := utils.Exists(makefilePath)
			if !makefileExists {
				makefilePath += ".win"
				makefileExists = utils.Exists(makefilePath)
			}

			if makefileExists {
				makeBin := ""

				if f, err := exec.LookPath("make"); err == nil {
					makeBin = f
				} else if f, err = exec.LookPath("nmake"); err == nil {
					makeBin = f
				}

				if makeBin != "" {
					runCmd = utils.Command(makeBin, "run")
				}
			}
		}

		if runCmd == nil {
			runCmd = utils.Command("go", "run", ".")
		}
		runCmd.Dir = projectPath
	} else {
		runCmd = utils.Command("go", "run", projectPath)
	}

	runCmd.Stdout = stdOut
	runCmd.Stderr = stdErr
	return runCmd.Run()
}

func (p *Project) Install() error {
	b, err := p.download()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			// Remove any installed files on errors.
			err = p.Unistall()
		}
	}()

	err = p.unzip(b)
	if err != nil {
		return err
	}

	err = p.build()
	if err != nil {
		return err
	}

	return err
}

func (p *Project) download() ([]byte, error) {
	p.Version = strings.Split(p.Version, " ")[0]
	if p.Version == "latest" {
		p.Version = "master"
	}

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

		p.Files = append(p.Files, fpath)

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

	return nil
}

const nodeModulesName = "node_modules"

func (p *Project) build() error {
	files, err := utils.FindMatches(p.Dest, "package.json")
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	npmBin, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("project %s requires nodejs to be installed", p.Name)
	}

	for _, f := range files {
		// check if not exist, if exists then do nothing otherwise run "npm install" automatically.
		dir := filepath.Dir(f)
		if strings.Contains(dir, nodeModulesName) {
			// it is a sub module which is already installed.
			continue
		}

		nodeModulesFolder := filepath.Join(dir, nodeModulesName)
		if utils.Exists(nodeModulesFolder) {
			continue
		}

		cmd := utils.Command(npmBin, "install")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return errors.New(string(out))
		}
	}

	return nil
}

// Unistall removes all project-associated files.
// it cannot run on a new session(maybe a TODO); the "p.Files" should be filles by `Install`.
func (p *Project) Unistall() (err error) {
	for _, f := range p.Files {
		if err = os.RemoveAll(f); err != nil {
			return
		}
	}

	return
}
