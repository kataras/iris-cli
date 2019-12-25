package cmd

import (
	"sort"
	"strings"

	"github.com/kataras/iris-cli/snippet"
	"github.com/kataras/iris-cli/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

const defaultRepo = "iris-contrib/snippets"

// iris-cli add --repo=kataras/golog logger.go@v0.0.10
// iris-cli add logger.go
// iris-cli add --repo=kataras/golog
func addCommand() *cobra.Command {
	var file = snippet.File{
		Repo:    defaultRepo,
		Version: "master",
		Dest:    "./",
		Package: "",
	}

	cmd := &cobra.Command{
		Use:           "add",
		Short:         "Add generates a file",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				cmd.Printf("Loading snippets from <%s>\n", file.Repo)
				files, err := snippet.ListFiles(file.Repo, file.Version)
				if err != nil {
					return err
				}

				availableSnippets := make([]string, 0, len(files))
				for _, f := range files {
					availableSnippets = append(availableSnippets, f.Name)
				}
				sort.Strings(availableSnippets)
				sort.Slice(availableSnippets, func(i, j int) bool {
					if strings.HasSuffix(availableSnippets[i], ".go") {
						return true
					}

					return false
				})

				qs := []*survey.Question{
					{
						Name:   "Name",
						Prompt: &survey.Select{Message: "Select snippet:", Options: availableSnippets, PageSize: 15},
					},
					{
						Name: "Package",
						Prompt: &survey.Input{Message: "What should be the new package name?",
							Help: "Leave it empty to be resolved automatically by your current project's files"},
					},
					{
						Name:   "Dest",
						Prompt: &survey.Input{Message: "Choose a file or directory to be saved:", Default: file.Dest},
					},
				}

				if err := survey.Ask(qs, &file); err != nil {
					return err
				}
			} else {
				file.Name, file.Version = utils.SplitNameVersion(args[0])
			}

			return file.Install()
		},
	}

	cmd.Flags().StringVar(&file.Repo, "repo", file.Repo, "--repo=iris-contrib/snippets")
	cmd.Flags().StringVar(&file.Package, "pkg", file.Package, "--pkg=mypackage")
	cmd.Flags().StringVar(&file.Dest, "dest", file.Dest, "--dest=./")

	return cmd
}
