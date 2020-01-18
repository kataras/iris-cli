package project

import (
	"archive/zip"
	"bytes"
	"context"
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

	"github.com/kataras/golog"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

type Project struct {
	Name string `json:"name,omitempty" yaml:"Name" toml:"Name"` // e.g. starter-kit
	// Remote.
	Repo    string `json:"repo" yaml:"Repo" toml:"Repo"`                    // e.g. "iris-contrib/starter-kit"
	Version string `json:"version,omitempty" yaml:"Version" toml:"Version"` // if empty then set to "master"
	// Local.
	Dest         string            `json:"dest,omitempty" yaml:"Dest" toml:"Dest"`       // if empty then $GOPATH+Module or ./+Module, absolute path of project destination.
	Module       string            `json:"module,omitempty" yaml:"Module" toml:"Module"` // if empty then set to the remote module name fetched from go.mod
	Replacements map[string]string `json:"-" yaml:"-" toml:"-"`                          // any raw text replacements.
	// Pre Installation.
	Reader func(io.Reader) ([]byte, error) `json:"-" yaml:"-" toml:"-"`
	// Post installation.
	// DisableInlineCommands disables source code comments stats with // $ _command_ to execute on "run" command.
	DisableInlineCommands bool `json:"disable_inline_commands" yaml:"DisableInlineCommands" toml:"DisableInlineCommands"`
	// DisableNpmInstall if `Run` and watch should NOT run npm install on first ran (and package.json changes).
	// Defaults to false.
	DisableNpmInstall bool `json:"disable_npm_install" yaml:"DisableNpmInstall" toml:"DisableNpmInstall"`
	// NpmBuildScriptName the package.json -> scripts[name] to execute on run and frontend changes.
	// Defaults to "build".
	NpmBuildScriptName string `json:"npm_build_script_name" yaml:"NpmBuildScriptName" toml:"NpmBuildScriptName"`
	// DisableWatch set to true to disable re-building and re-run the server and its frontend assets on file changes after first `Run`.
	DisableWatch bool        `json:"disable_watch" yaml:"DisableWatch" toml:"DisableWatch"`
	LiveReload   *LiveReload `json:"livereload" yaml:"LiveReload" toml:"LiveReload"`

	// Relative path of the files and directories installed, because the folder may be not empty
	// and when installation fails we don't want to delete any user-defined files,
	// just the project's ones before build.
	Files          []string `json:"files,omitempty" yaml:"Files" toml:"Files"`
	BuildFiles     []string `json:"build_files" yaml:"BuildFiles" toml:"BuildFiles"` // New directories and files, relatively to p.Dest, that are created by build (makefile, build script, npm install & npm run build).
	MD5PackageJSON []byte   `json:"md5_package_json" yaml:"MD5PackageJSON" toml:"MD5PackageJSON"`

	runner *exec.Cmd

	// TODO:
	// Running is set automatically to true on `Run` and false on interrupt,
	// it is used for third-parties software to check if a specific project is run under iris-cli.
	Running        bool `json:"running" yaml:"Running,omitempty" toml:"Running"`
	stdout, stderr io.Writer

	// runningCommands chan context.CancelFunc
	frontEndRunningCommands map[*exec.Cmd]context.CancelFunc
}

const ProjectFilename = ".iris.yml"

func (p *Project) setDefaults() {
	if p.LiveReload == nil {
		p.LiveReload = NewLiveReload()
	}

	if p.DisableWatch {
		// fixes configuration file if Project.Watch is not enabled
		// but Project.LiveReload is enabled.
		p.LiveReload.Disable = true
	}

	p.frontEndRunningCommands = make(map[*exec.Cmd]context.CancelFunc) // make(chan context.CancelFunc, 20)
}

func (p *Project) SaveToDisk() error {
	p.setDefaults()

	projectFile := filepath.Join(p.Dest, ProjectFilename)

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

	projectFile := filepath.Join(projectPath, ProjectFilename)
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

	p.setDefaults()
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
	)

	p.Dest = utils.Dest(p.Dest)

	for _, f := range r.File {
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
		if shouldReplaceModule, hasReplacements := !bytes.Equal(oldModuleName, newModuleName), len(p.Replacements) > 0; shouldReplaceModule || hasReplacements {
			contents, ioErr := ioutil.ReadAll(rc)
			if ioErr != nil {
				return ioErr
			}

			if shouldReplaceModule {
				contents = bytes.ReplaceAll(contents, oldModuleName, newModuleName)
			}

			if hasReplacements {
				for oldContent, newContent := range p.Replacements {
					if !shouldReplaceModule {
						// If username/repo style then update go module too.
						if key := "github.com/" + oldContent; key == p.Module {
							newModuleName = append([]byte("github.com/"), newContent...)
							contents = bytes.ReplaceAll(contents, oldModuleName, newModuleName)
							p.Module = string(newModuleName)
							shouldReplaceModule = true // once.
						}
					}

					contents = bytes.ReplaceAll(contents, []byte(oldContent), []byte(newContent))
				}
			}

			_, err = outFile.Write(contents)
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

func (p *Project) Run(stdout, stderr io.Writer) error {
	p.stdout = stdout
	p.stderr = stderr

	err := p.build()
	if err != nil {
		return err
	}

	var g errgroup.Group

	if err := p.start(); err != nil {
		return err
	}

	g.Go(p.runner.Wait)
	if !p.DisableWatch {
		g.Go(p.LiveReload.ListenAndServe)
		g.Go(p.watch)
	}

	return g.Wait()
}

func (p *Project) start() error {
	if runCmd := getActionCommand(p.Dest, ActionRun); runCmd != nil {
		runCmd.Dir = p.Dest
		runCmd.Stdout = p.stdout
		runCmd.Stderr = p.stderr
		if err := runCmd.Start(); err != nil {
			return err
		}

		p.runner = runCmd
		return nil
	}

	bin := utils.FormatExecutable(filepath.Base(p.Dest))
	buildCmd := utils.Command("go", "build", "-o", bin, ".")
	buildCmd.Dir = p.Dest
	if b, err := buildCmd.CombinedOutput(); err != nil {
		return errors.New(err.Error() + "\n" + string(b)) // don't use fmt.Errorf here for any case that the format contains vars.
	}

	runCmd, err := utils.StartExecutable(p.Dest, bin, p.stdout, p.stderr)
	if err != nil {
		return err
	}

	p.runner = runCmd
	return nil
}

const nodeModulesName = "node_modules"

type packageJSON struct {
	// Name            string            `json:"name"`
	// Version         string            `json:"version"`
	// Dependencies    map[string]string `json:"dependencies"`
	// DevDependencies map[string]string `json:"devDependencies"`
	Scripts map[string]string `json:"scripts"`
}

func (p *Project) rel(name string) string {
	rel, err := filepath.Rel(p.Dest, name)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

func (p *Project) build() error {
	// Add any new directories and files to build files and save the project on built.
	watcher, err := utils.NewWatcher()
	if err != nil {
		return fmt.Errorf("build: watcher: %s: %v", p.Dest, err)
	}

	watcher.AddRecursively(p.Dest)
	go func() {
		for {
			select {
			case evts := <-watcher.Events:
				for _, evt := range evts {
					name := p.rel(evt.Name)

					// fmt.Printf("| %s | %s\n", evt.Op.String(), name)

					switch evt.Op {
					case utils.FileCreate:
						exists := false
						for _, buildName := range p.BuildFiles {
							if buildName == name {
								exists = true
								break
							}
						}

						if !exists {
							p.BuildFiles = append(p.BuildFiles, name)
						}

					case utils.FileRemove:
						for i, buildName := range p.BuildFiles {
							if buildName == name {
								copy(p.BuildFiles[i:], p.BuildFiles[i+1:])
								p.BuildFiles[len(p.BuildFiles)-1] = ""
								p.BuildFiles = p.BuildFiles[:len(p.BuildFiles)-1]
								break
							}
						}
					}
				}
			}
		}
	}()

	defer p.SaveToDisk()
	defer watcher.Close()

	// newFilesFn, err := utils.GetFilesDiff(p.Dest)
	// if err != nil {
	// 	return err
	// }

	// Try to build with "make", "nmake" or "build.bat", "build.sh".
	buildCmd := getActionCommand(p.Dest, ActionBuild)
	if buildCmd != nil {
		return runCmd(buildCmd, p.Dest)

		// if buildFiles := newFilesFn(); len(buildFiles) > 0 {
		// 	p.BuildFiles = buildFiles
		// }

		// return p.SaveToDisk()
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
			return fmt.Errorf("%s: package.json: %v", ActionBuild, err)
		}

		if !p.DisableNpmInstall {
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
				installCmd, cancelFunc := utils.CommandWithCancel(npmBin, "install")
				p.frontEndRunningCommands[installCmd] = cancelFunc
				// defer cancelFunc()
				if err = runCmd(installCmd, dir); err != nil {
					return err
				}
			}
		}

		if p.NpmBuildScriptName != "" {
			// Check if package.json contains a build action and run it.
			var v packageJSON
			if err = json.Unmarshal(b, &v); err != nil {
				return fmt.Errorf("%s: package.json: %v", ActionBuild, err)
			}

			if _, ok := v.Scripts[ActionBuild]; ok {
				buildCmd, cancelFunc := utils.CommandWithCancel(npmBin, "run", ActionBuild)
				p.frontEndRunningCommands[buildCmd] = cancelFunc
				// defer cancelFunc()
				if err = runCmd(buildCmd, dir); err != nil {
					return err
				}
			}
		}
	}

	// after npm install and npm build.
	res, err := parser.Parse(p.Dest)
	if err == nil {

		skipGenerateAssetsIndexes := make(map[int]struct{})

		if !p.DisableInlineCommands {
			for _, c := range res.Commands {
				cmd, cancelFunc := utils.CommandWithCancel(c.Name, c.Args...)
				// Author's Note:
				// track the executed commands: if go-bindata related
				// with the same res.AssetDirs[x] then skip the manual go-bindata command execution
				// which follows after <TODO>.
				if !utils.Exists(c.Dir) {
					cmd.Dir = p.Dest
				}

				if c.Name == "go-bindata" {
					for _, arg := range c.Args {
						for i, assetDir := range res.AssetDirs {
							if assetDir.ShouldGenerated && filepath.ToSlash(assetDir.Dir+"/...") == arg {
								// a custom command generates those assets.
								skipGenerateAssetsIndexes[i] = struct{}{}
							}
						}
					}
				}

				p.frontEndRunningCommands[cmd] = cancelFunc
				// defer cancelFunc()
				if err = runCmd(cmd, ""); err != nil {
					return fmt.Errorf("command <%s> failed:\n%v", c.Name, err)
				}
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
			goBindata, cancelFunc := utils.CommandWithCancel("go-bindata", args...)
			p.frontEndRunningCommands[goBindata] = cancelFunc
			// defer cancelFunc()
			if err = runCmd(goBindata, p.Dest); err != nil {
				return err
			}
		}
	}

	// Add any new directories and files to build files and save the project on built.
	// p.BuildFiles = append(p.BuildFiles, newFilesFn()...)
	// if buildFiles := newFilesFn(); len(buildFiles) > 0 {
	// 	p.BuildFiles = buildFiles
	// }

	// return p.SaveToDisk()

	return nil
}

// TODO: not only rebuild frontend and reload server-side but add a browser live reload through a different websocket server (here)
// which an iris app's frontend javascript file should communicate, this can be happen through Iris side configuration to disable/enable it
// and let iris-cli parser take action based on that(?). A good start is to implement the LiveReload protocol: http://livereload.com/api/protocol/
// or make a custom and fairly simple module for that.
func (p *Project) watch() error {
	println(`+-------------------------------------------------+
|                                                 |
|      ___ ____  ___ ____     ____ _     ___      |
|     |_ _|  _ \|_ _/ ___|   / ___| |   |_ _|     |
|      | |+ |_) || |\___ \  | |   | |    | |      |
|      | |+  _ < | | ___) | | |___| |___ | |      |
|     |___|_| \_\___|____/   \____|_____|___|     |
|                                                 |
|                                                 |
|                                                 |
|                                                 |
|                             https://iris-go.com |
+-------------------------------------------------+
`)

	watcher, err := utils.NewWatcher()
	if err != nil {
		return fmt.Errorf("watch: watcher: %s: %v", p.Dest, err)
	}

	watcher.AddFilter = func(dir string) bool {
		dir = strings.TrimPrefix(dir, p.Dest+"/")
		if dir == p.Dest {
			return true
		}

		// If it's a build directory should be ignored by the watcher.
		for _, f := range p.BuildFiles {
			if strings.HasPrefix(dir, f) {
				return false
			}
		}

		return true
	}

	watcher.AddRecursively(p.Dest)

	for _, dir := range watcher.Dirs {
		dir = strings.TrimPrefix(dir, p.Dest)
		golog.Infof("Watching %s/*", dir)
	}

	// serving := new(uint32)

	rerun := func(frontend, backend bool) (err error) {
		watcher.Pause()
		defer watcher.Continue()

		desc := ""
		if frontend {
			desc += "√ Frontend"
		}
		if backend {
			if desc != "" {
				desc += " "
			}
			desc += "√ Backend"
		}
		golog.Infof("Change detected [%s]", desc)

		// for !atomic.CompareAndSwapUint32(serving, 0, 1) {
		// 	time.Sleep(25 * time.Millisecond)
		// }

		defer func() {
			// atomic.StoreUint32(serving, 0)
			if err == nil {
				p.LiveReload.SendReloadSignal()
			}
		}()

		if frontend {
			for cmd, cancelFunc := range p.frontEndRunningCommands {
				cancelFunc()
				delete(p.frontEndRunningCommands, cmd)
				cmd = nil
			}

			if err = p.build(); err != nil {
				return err
			}
		}

		if backend {
			utils.KillCommand(p.runner)
			// timeout := time.Second // give some time to release the TCP server port.
			// for conn, _ := net.DialTimeout("tcp", ":8080", timeout); conn != nil; {
			// 	// still open.
			// 	conn.Close()
			// 	time.Sleep(25 * time.Millisecond)
			// }

			err = p.start()
			if err == nil {
				// TODO: find a way to get the iris app's listening port in order to support
				// port navigation too on browser live reload feature.
				// First, use go build -o currentdir+exec_ext . to have a static path of the executable file and its name
				// and also give the ability for the external iris app to use relative paths for file conf files and e.t.c.
				//
				// 1. instead of using cmd /c and /bin/sh -c to start the program, run it directly
				// that way we have the correct p.runner.Process.Pid and not its parent, however that may fail due permission issues on unix (not tested yet but I assume).
				// OR
				//  2. use that executable name to get the proc ID
				//   2.1 use that proc ID to get and parse the listening PORT
				/*
					using "github.com/keybase/go-ps"
						bin := utils.FormatExecutable(filepath.Base(p.Dest))

						time.Sleep(1 * time.Second)

						procs, _ := ps.Processes()
						for _, proc := range procs {
							println(proc.Executable())
							if proc.Executable() == bin {
								println("========= FOUND ========")
								println(proc.Pid())
							}
						}

						 want to get the port listening through:
							C:\Users\kataras>netstat -a -n -p tcp -o | find "7104"
							TCP    0.0.0.0:9080           0.0.0.0:0              LISTENING       7104

					This works but I don't want to use time.Sleep just to wait from
					cmd /c or /bin/sh -c shells to fork and start the process of our executable file.
				*/
				// OR
				// 3. let iris tell us what it's port by creating a temp file in the current working directory
				// or by changing the .iris.yml configuration file itself to a Running: Port: $PORT and then
				// let live reloader read it and send it to the client side of the app.
				//
				// Maybe browser live reload on backend addr/port changing does not worth such a waste of time.
			}
		}

		return
	}

	for {
		select {
		case <-watcher.Closed():
			return nil
		case evts := <-watcher.Events:
			// if many events, just build the whole project.
			if len(evts) > 20 {
				go rerun(true, true)
				continue
			}

			backendChanged := false
			frontendChanged := false

			// TODO: if the process is slow we must collect more events until build process finishes
			// (or cancel the previous with exec.Command with context?)

			for _, evt := range evts {
				name := p.rel(evt.Name)
				// fmt.Printf("| %s | %s\n", evt.Op.String(), name)

				ext := ""
				if idx := strings.LastIndexByte(name, '.'); idx > 0 && len(name)-1 > idx {
					ext = name[idx:]
				}

				if ext == "" {
					// skip...
					continue
				}

				switch ext {
				case ".go", ".mod":
					backendChanged = true
				case ".html", ".htm", ".svelte",
					".js", ".ts",
					".jsx", ".tsx",
					".css", ".scss", ".less",
					".json":
					frontendChanged = true
				case ".yml", ".toml", ".tml", ".ini":
					if name == ProjectFilename {
						// skip if it's the .iris.yml project file.
						continue
					}

					// probably a server configuration file.
					backendChanged = true
				case ".proto":
					frontendChanged = true
					backendChanged = true
				case ".exe", ".exe~", ".tmp":
					continue
				default:
					// sometimes something like "app/build" is changed while building, although
					// it's ignored by the watcher itself...and chmod is ignored, so:
					// for _, f := range p.BuildFiles {
					// 	if f == name {
					// 		continue eventsLoop
					// 	}
					// }
					// added ext == "" => skip ^
					golog.Warnf("Unexpected file %s(change=%s) changed, is this a frontend or backend change? Please report it to Github issues.", name, evt.Op.String())
					continue
				}
			}

			if frontendChanged || backendChanged {
				go rerun(frontendChanged, backendChanged)
			}
		}
	}
}

// Clean removes all project's build-only associated files.
func (p *Project) Clean() (err error) {
	for _, f := range p.BuildFiles {
		f = filepath.Join(p.Dest, f)
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
		f = filepath.Join(p.Dest, f)
		if err = os.RemoveAll(f); err != nil {
			return
		}
	}

	// remove go.sum (which can be automatically generated if not existed because of a remote project with .gitignore).
	goSumFile := filepath.Join(p.Dest, "go.sum")
	os.Remove(goSumFile) // ignore error.

	// try to remove executable.
	binFile := filepath.Join(p.Dest, utils.FormatExecutable(filepath.Base(p.Dest)))
	os.Remove(binFile)

	// remove project file too.
	projectFile := filepath.Join(p.Dest, ProjectFilename)
	return os.Remove(projectFile)
}

const (
	ActionRun   = "run"
	ActionBuild = "build"
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
