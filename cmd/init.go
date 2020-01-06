package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/kataras/iris-cli/parser"
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
		Short:         "Init creates the iris project file from a LOCAL git repository. Useful for custom projects",
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

			r, err := git.PlainOpen(projectPath)
			if err != nil {
				return fmt.Errorf("not git repository")
			}

			remotes, err := r.Remotes()
			if err != nil {
				return err
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

			cmd.Printf("Repo: %s\n", repo)

			// Find version, if any (otherwise it defaults to master)
			version, err := getLatestTagFromRepository(r)
			if version == "" {
				version, err = getCurrentBranchFromRepository(r)
			}

			if err != nil {
				return err
			}

			version = filepath.Base(version)

			cmd.Printf("Version: %s\n", version)

			// Find go module path.
			goModFile := filepath.Join(projectPath, "go.mod")
			if !utils.Exists(goModFile) {
				return fmt.Errorf("project missing <go.mod> file")
			}
			b, err := ioutil.ReadFile(goModFile)
			if err != nil {
				return err
			}

			module := string(parser.ModulePath(b))

			cmd.Printf("Module: %s\n", module)

			// t, err := r.Worktree()
			// if err != nil {
			// 	return err
			// }
			// patterns, _ := gitignore.ReadPatterns(t.Filesystem, nil)
			// patterns = append(patterns, t.Excludes...)
			// if len(patterns) > 0 {
			// 	m := gitignore.NewMatcher(patterns)

			var files []string

			ignore, err := gitignore.NewFromFile(filepath.Join(projectPath, ".gitignore"))
			if err != nil {
				return err
			}

			err = filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
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
				return err
			}

			cmd.Printf("\n\n\nFiles: %s\n", strings.Join(files, "\n"))

			// TODO: use t.Excludes to find build files and -projectfiles

			// TODO: create .iris project, repository and other information may be fetched from git repository (through .git folder).
			return nil
		},
	}
	return cmd
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
