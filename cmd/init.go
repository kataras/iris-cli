package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/kataras/iris-cli/parser"
	"github.com/kataras/iris-cli/project"
	"github.com/kataras/iris-cli/utils"

	"github.com/spf13/cobra"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"

	// this does not work: "gopkg.in/src-d/go-git.v4/plumbing/format/gitignore"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	// we use that instead.
	"github.com/denormal/go-gitignore"
)

func initCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "init",
		// The directory should not contain any build files/should be clean.
		Short:         "Init creates the iris project file from a git repository or local directory. Useful for custom projects",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 1 {
				path = args[1]
			}

			projectPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}

			name, repo, version, err := fromLocalGitRepository(projectPath)
			if err != nil {
				if err != errNotGitRepository {
					return err
				}

				name = filepath.Base(projectPath)
				repo = ""
				version = "main"
				// err = nil
			}

			files, err := findFiles(projectPath)
			if err != nil {
				if err != errGitIgnoreFileMissing {
					return err
				}
				// else all files.
				err = filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
					if path == projectPath {
						return nil // skip the project path itself ( we don't want to remove the folder itself on unistall, just its files).
					}

					if err != nil {
						return err
					}

					rel, err := filepath.Rel(projectPath, path)
					if err != nil {
						return err
					}

					files = append(files, filepath.ToSlash(rel))
					return nil
				})

				if err != nil {
					return err
				}
			}

			if len(files) == 0 {
				return fmt.Errorf("empty directory")
			}

			// Get go module path.
			module := findModulePath(projectPath)

			p := &project.Project{
				Name:               name,
				Repo:               repo,
				Version:            version,
				Dest:               filepath.ToSlash(projectPath),
				Module:             module,
				Files:              files, // if git repository then on unistall command the .git directory remains, if community wants to remove that too then will do.
				NpmBuildScriptName: project.ActionBuild,
			}

			return p.SaveToDisk()
		},
	}
	return cmd
}

var errNotGitRepository = fmt.Errorf("not a git repository")

func fromLocalGitRepository(projectPath string) (string, string, string, error) {
	r, err := git.PlainOpen(projectPath)
	if err != nil {
		return "", "", "", errNotGitRepository
	}

	remotes, err := r.Remotes()
	if err != nil {
		return "", "", "", err
	}

	// Find github remote repo, if any.
	repo := ""
	for _, remote := range remotes {
		c := remote.Config()
		for i, u := range c.URLs {
			if c.IsFirstURLLocal() && i == 0 {
				continue
			}

			if !strings.Contains(u, "github.com") {
				continue
			}

			repo = strings.TrimPrefix(strings.TrimSuffix(u, ".git"), "https://github.com/")
			break
		}

		if repo != "" {
			break
		}
	}

	// Find version, if any (otherwise it defaults to master/main)
	version, err := getLatestTagFromRepository(r)
	if version == "" {
		version, err = getCurrentBranchFromRepository(r)
	}

	if err != nil {
		return "", "", "", err
	}

	version = filepath.Base(version)

	return filepath.Base(repo), repo, version, nil
}

var errGitIgnoreFileMissing = fmt.Errorf(".gitignore is missing")

func findFiles(projectPath string) ([]string, error) {
	// Read for build files at a .gitignore (even if this is not a git fully project)
	gitIgnoreFile := filepath.Join(projectPath, ".gitignore")
	if !utils.Exists(gitIgnoreFile) {
		return nil, errGitIgnoreFileMissing
	}

	ignore, err := gitignore.NewFromFile(gitIgnoreFile)
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == projectPath {
			// skip root itself.
			return nil
		}

		rel, err := filepath.Rel(projectPath, path)
		if err != nil {
			return err
		}

		if rel == ".git" {
			// ignore .git folder.
			return filepath.SkipDir
		}

		rel = filepath.ToSlash(rel)

		isDir := info.IsDir()
		if m := ignore.Relative(rel, isDir); m != nil && m.Ignore() {
			if isDir {
				return filepath.SkipDir
			}

			return nil
		}

		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if m := ignore.Relative(project.ProjectFilename, false); m == nil { // if entry does not exist at all. Skip if included by user.
		// then add it to the gitignore.
		f, err := os.OpenFile(gitIgnoreFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			_, err = f.WriteString("\n# ignore iris project file\n.iris.yml\n")
			f.Close()
			if err != nil {
				return nil, err
			}
		}
	}

	return files, nil
}

func findModulePath(projectPath string) string {
	goModFile := filepath.Join(projectPath, "go.mod")
	if !utils.Exists(goModFile) {
		return ""
	}
	b, err := ioutil.ReadFile(goModFile)
	if err != nil {
		return ""
	}

	return string(parser.ModulePath(b))
}

func getCurrentBranchFromRepository(repository *git.Repository) (string, error) {
	branchRefs, err := repository.Branches()
	if err != nil {
		return "", err
	}

	headRef, err := repository.Head()
	if err != nil {
		return "", err
	}

	var currentBranchName string
	err = branchRefs.ForEach(func(branchRef *plumbing.Reference) error {
		if branchRef.Hash() == headRef.Hash() {
			currentBranchName = branchRef.Name().String()

			return nil
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return currentBranchName, nil
}

func getCurrentCommitFromRepository(repository *git.Repository) (string, error) {
	headRef, err := repository.Head()
	if err != nil {
		return "", err
	}
	headSha := headRef.Hash().String()

	return headSha, nil
}

func getLatestTagFromRepository(repository *git.Repository) (string, error) {
	tagRefs, err := repository.Tags()
	if err != nil {
		return "", err
	}

	var latestTagCommit *object.Commit
	var latestTagName string
	err = tagRefs.ForEach(func(tagRef *plumbing.Reference) error {
		revision := plumbing.Revision(tagRef.Name().String())
		tagCommitHash, err := repository.ResolveRevision(revision)
		if err != nil {
			return err
		}

		commit, err := repository.CommitObject(*tagCommitHash)
		if err != nil {
			return err
		}

		if latestTagCommit == nil {
			latestTagCommit = commit
			latestTagName = tagRef.Name().String()
		}

		if commit.Committer.When.After(latestTagCommit.Committer.When) {
			latestTagCommit = commit
			latestTagName = tagRef.Name().String()
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return latestTagName, nil
}
