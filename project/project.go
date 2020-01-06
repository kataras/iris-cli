package project

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kataras/iris-cli/parser"
	"github.com/kataras/iris-cli/utils"

	"gopkg.in/yaml.v3"
)

type Project struct {
	Name string `json:"name,omitempty" yaml:"Name" toml:"Name"` // e.g. starter-kit
	// Remote.
	Repo    string `json:"repo" yaml:"Repo" toml:"Repo"`                    // e.g. "iris-contrib/starter-kit"
	Version string `json:"version,omitempty" yaml:"Version" toml:"Version"` // if empty then set to "master"
	// Local.
	Dest   string `json:"dest,omitempty" yaml:"Dest" toml:"Dest"`       // if empty then $GOPATH+Module or ./+Module, absolute path of project destination.
	Module string `json:"module,omitempty" yaml:"Module" toml:"Module"` // if empty then set to the remote module name fetched from go.mod
	// Pre Installation.
	Reader func(io.Reader) ([]byte, error) `json:"-" yaml:"-" toml:"-"`
	// Post installation.
	// Relative path of the files and directories installed, because the folder may be not empty
	// and when installation fails we don't want to delete any user-defined files,
	// just the project's ones before build.
	Files      []string `json:"files,omitempty" yaml:"Files" toml:"Files"`
	BuildFiles []string `json:"build_files" yaml:"BuildFiles" toml:"BuildFiles"` // New directories and files, relatively to p.Dest, that are created by build (makefile, build script, npm install & npm run build).

	MD5PackageJSON []byte `json:"md5_package_json" yaml:"MD5PackageJSON" toml:"MD5PackageJSON"`
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

const projectFilename = ".iris.yml"

func (p *Project) SaveToDisk() error {
	projectFile := filepath.Join(p.Dest, projectFilename)

	outFile, err := os.OpenFile(projectFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer outFile.Close()

	enc := yaml.NewEncoder(outFile)
	return enc.Encode(p)
	// enc := gob.NewEncoder(outFile)
	// return enc.Encode(p)
}

var ErrProjectFileNotExist = errors.New("project file does not exist")

func LoadFromDisk(path string) (*Project, error) {
	projectPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if !utils.Exists(projectPath) {
		return nil, ErrProjectNotExists
	}

	if !utils.IsDir(projectPath) {
		projectPath = filepath.Dir(projectPath)
	}

	projectFile := filepath.Join(projectPath, projectFilename)
	if !utils.Exists(projectFile) {
		return nil, ErrProjectFileNotExist
	}

	inFile, err := os.OpenFile(projectFile, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, err
	}
	defer inFile.Close()

	p := new(Project)
	// dec := gob.NewDecoder(inFile)
	// err = dec.Decode(p)
	dec := yaml.NewDecoder(inFile)
	err = dec.Decode(p)
	if err != nil {
		return nil, err
	}

	return p, nil
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

	// Do not build on Install,
	// Build is another step
	// and it runs automatically on Run if was not built
	// (TODO: or source code changed).
	//
	// err = p.build()
	// if err != nil {
	// 	return err
	// }

	return p.SaveToDisk()
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

			oldModuleName = parser.ModulePath(contents)
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
		// name := strings.TrimPrefix(f.Name, compressedRootFolder)
		// if name == "" {
		// 	// root folder.
		// 	continue
		// }
		rel, err := filepath.Rel(compressedRootFolder, f.Name)
		if err != nil {
			continue
		}
		if rel == "." {
			continue
		}
		name := filepath.ToSlash(rel)
		fpath := path.Join(p.Dest, name)

		// // https://snyk.io/research/zip-slip-vulnerability#go
		// if !strings.HasPrefix(fpath, p.Dest+"/") {
		// 	return fmt.Errorf("illegal path: %s", fpath)
		// }

		if f.FileInfo().IsDir() {
			p.Files = append(p.Files, name)

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

		// if err = os.Chtimes(fpath, modTime, modTime); err != nil {
		// 	return err
		// }

		p.Files = append(p.Files, name)
	}

	return nil
}

func (p *Project) Run(stdOut, stdErr io.Writer) error {
	if err := p.build(); err != nil {
		return err
	}

	runCmd := getActionCommand(p.Dest, actionRun)
	if runCmd == nil {
		runCmd = utils.Command("go", "run", ".")
	}

	runCmd.Dir = p.Dest
	runCmd.Stdout = stdOut
	runCmd.Stderr = stdErr

	return runCmd.Run()
}

const nodeModulesName = "node_modules"

type packageJSON struct {
	// Name            string            `json:"name"`
	// Version         string            `json:"version"`
	// Dependencies    map[string]string `json:"dependencies"`
	// DevDependencies map[string]string `json:"devDependencies"`
	Scripts map[string]string `json:"scripts"`
}

func (p *Project) build() error {
	// Add any new directories and files to build files and save the project on built.
	onFileChange := func(filename string) {
		/* e.g.
		build file: C:\Users\kataras\Desktop\myproject\app\node_modules
		build file: C:\Users\kataras\Desktop\myproject\app\package-lock.json.545868579
		build file: C:\Users\kataras\Desktop\myproject\app\package-lock.json
		build file: C:\Users\kataras\Desktop\myproject\app\public\build
		*/
		rel, err := filepath.Rel(p.Dest, filename)
		if err == nil {
			p.BuildFiles = append(p.BuildFiles, filepath.ToSlash(rel))
		}
	}
	onFileRemove := func(filename string) {
		for i, name := range p.BuildFiles {
			if name == filename {
				copy(p.BuildFiles[i:], p.BuildFiles[i+1:])
				p.BuildFiles[len(p.BuildFiles)-1] = ""
				p.BuildFiles = p.BuildFiles[:len(p.BuildFiles)-1]
			}
		}
	}
	watcher, err := utils.WatchFileChanges(p.Dest, utils.WatchFileEvents{utils.FileCreate: onFileChange, utils.FileRemove: onFileRemove})

	if err != nil {
		return fmt.Errorf("watcher <%s>: %v", p.Dest, err)
	}

	defer watcher.Close()

	// Try to build with "make", "nmake" or "build.bat", "build.sh".
	buildCmd := getActionCommand(p.Dest, actionBuild)
	if buildCmd != nil {
		return runCmd(buildCmd, p.Dest)
	}

	// Locate any package.json project files and
	// npm install. Afterwards npm run build if scripts: "build" exists.
	files, err := utils.FindMatches(p.Dest, "package.json", "*\\node_modules", false)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	npmBin, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("project <%s> requires nodejs to be installed", p.Name)
	}

	for _, f := range files {
		// Check if not exist, if exists then do nothing otherwise run "npm install" automatically.
		dir := filepath.Dir(f)

		if strings.Contains(dir, nodeModulesName) {
			// It is a sub module which is already installed.
			continue
		}

		b, err := ioutil.ReadFile(f)
		if err != nil {
			return fmt.Errorf("%s: package.json: %v", actionBuild, err)
		}

		// Run "npm install" only when package.json changed since last build
		// or when node_modules missing.
		shouldNpmInstall := false
		if md5b := md5.Sum(b); !bytes.Equal(p.MD5PackageJSON, md5b[:]) {
			p.MD5PackageJSON = md5b[:]
			shouldNpmInstall = true
		}

		if !utils.Exists(filepath.Join(dir, nodeModulesName)) {
			shouldNpmInstall = true
		}

		if shouldNpmInstall {
			installCmd := utils.Command(npmBin, "install")
			if err = runCmd(installCmd, dir); err != nil {
				return err
			}
		}

		// Check if package.json contains a build action and run it.
		var v packageJSON
		if err = json.Unmarshal(b, &v); err != nil {
			return fmt.Errorf("%s: package.json: %v", actionBuild, err)
		}

		if _, ok := v.Scripts[actionBuild]; ok {
			buildCmd := utils.Command(npmBin, "run", actionBuild)
			if err = runCmd(buildCmd, dir); err != nil {
				return err
			}
		}
	}

	// after npm install and npm build.
	res, err := parser.Parse(p.Dest)
	if err == nil {

		skipGenerateAssetsIndexes := make(map[int]struct{})

		for _, cmd := range res.Commands {
			// Author's Note:
			// track the executed commands: if go-bindata related
			// with the same res.AssetDirs[x] then skip the manual go-bindata command execution
			// which follows after <TODO>.
			if !utils.Exists(cmd.Dir) {
				cmd.Dir = p.Dest
			}

			commandName := cmd.Args[0]

			if commandName == "go-bindata" {
				if len(cmd.Args) > 1 {
					args := cmd.Args[1:]
					for _, arg := range args {
						for i, assetDir := range res.AssetDirs {
							if assetDir.ShouldGenerated && filepath.ToSlash(assetDir.Dir+"/...") == arg {
								// a custom command generates those assets.
								skipGenerateAssetsIndexes[i] = struct{}{}
							}
						}
					}
				}
			}

			if err = runCmd(cmd, ""); err != nil {
				return fmt.Errorf("command <%s> failed:\n%v", commandName, err)
			}
		}

		var dirsToBuild []string
		for i, assetDir := range res.AssetDirs {
			if _, skip := skipGenerateAssetsIndexes[i]; skip {
				continue
			}

			if !assetDir.ShouldGenerated {
				continue
			}

			dirsToBuild = append(dirsToBuild, filepath.ToSlash(assetDir.Dir+"/..."))
		}

		if len(dirsToBuild) > 0 {
			args := append([]string{
				"-o",
				"bindata.go",
			}, dirsToBuild...)
			goBindata := utils.Command("go-bindata", args...)
			if err = runCmd(goBindata, p.Dest); err != nil {
				return err
			}
		}
	}

	return p.SaveToDisk()
}

// Clean removes all project's build-only associated files.
func (p *Project) Clean() (err error) {
	for _, f := range p.BuildFiles {
		if err = os.RemoveAll(f); err != nil {
			return
		}
	}

	p.BuildFiles = nil
	return p.SaveToDisk()
}

// Unistall removes all project-associated files.
func (p *Project) Unistall() (err error) {
	if err = p.Clean(); err != nil {
		return
	}

	for _, f := range p.Files {
		if err = os.RemoveAll(f); err != nil {
			return
		}
	}

	// remove go.sum (which can be automatically generated if not existed because of a remote project with .gitignore).
	goSumFile := filepath.Join(p.Dest, "go.sum")
	os.Remove(goSumFile) // ignore error.

	// remove project file too.
	projectFile := filepath.Join(p.Dest, projectFilename)
	return os.Remove(projectFile)
}

const (
	actionRun   = "run"
	actionBuild = "build"
)

func getActionCommand(path string, action string) *exec.Cmd {
	if !utils.IsDir(path) {
		return nil
	}

	runScriptExt := ".bat"
	if runtime.GOOS != "windows" {
		runScriptExt = ".sh"
	}

	if runScriptPath := filepath.Join(path, action+runScriptExt); utils.Exists(runScriptPath) {
		// run.bat or run.sh exists
		return utils.Command(runScriptPath)
	}
	// else check for Makefile(make) or Makefile.win (nmake).
	makefilePath := filepath.Join(path, "Makefile")
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
			return utils.Command(makeBin, action)
		}
	}

	return nil
}

var thirdPartyBinaries = map[string]string{ // key = %GOPATH%/bin/$binary value = repository to fetch if not exists.
	"go-bindata": "github.com/go-bindata/go-bindata/...",
}

func runCmd(cmd *exec.Cmd, dir string) error {
	if dir != "" {
		cmd.Dir = dir
	}

	name := cmd.Args[0]
	if repo, ok := thirdPartyBinaries[name]; ok {
		if _, err := exec.LookPath(name); err != nil {
			// try go-get it.
			if err = runCmd(utils.Command("go", "get", "-u", "-f", repo), cmd.Dir); err != nil {
				return err
			}

			// This doesn't work because of unexported cmd.lookPathErr, so call `runCmd` again:
			// keep cmd.Args[0] as it's; it should be the base name without extension.
			// cmd.Path, err = exec.LookPath(name)
			// if err != nil {
			// 	return err
			// }

			// if !utils.Exists(cmd.Path) {
			// 	panic(cmd.Path + " does not exist after go get")
			// }

			var args []string
			if len(cmd.Args) > 1 {
				args = cmd.Args[1:]
			}

			return runCmd(utils.Command(name, args...), cmd.Dir)
		}
	}

	// println("Run: " + strings.Join(cmd.Args, " "))

	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(string(out))
	}

	return nil
}
