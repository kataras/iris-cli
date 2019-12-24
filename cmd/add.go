package cmd

import (
	"sort"
	"strings"

	"github.com/kataras/iris-cli/file"
	"github.com/kataras/iris-cli/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

const defaultRepo = "iris-contrib/snippets"

func addCommand() *cobra.Command {
	var (
		opts = struct {
			Repo    string
			Version string
			Snippet string
			Pkg     string
			Dest    string
		}{
			Repo:    defaultRepo,
			Version: "master",
			Snippet: "",
			Pkg:     "",
			Dest:    "./",
		}
	)

	cmd := &cobra.Command{
		Use:           "add",
		Short:         "Add generates a file",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Snippet, opts.Version = utils.SplitNameVersion(opts.Snippet)
			cmd.Printf("Loading snippets from <%s@%s>\n", opts.Repo, opts.Version)
			files, err := file.ListFiles(opts.Repo, opts.Version)
			if err != nil {
				return err
			}

			if len(args) == 0 {
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
						Name:   "snippet",
						Prompt: &survey.Select{Message: "Select snippet:", Options: availableSnippets, PageSize: 15},
					},
					{
						Name: "pkg",
						Prompt: &survey.Input{Message: "What should be the new package name?",
							Help: "Leave it empty to be resolved automatically by your current project's files"},
					},
					{
						Name:   "dest",
						Prompt: &survey.Input{Message: "Choose a file or directory to be saved:", Default: opts.Dest},
					},
				}

				if err := survey.Ask(qs, &opts); err != nil {
					return err
				}
			} else {
				opts.Snippet, opts.Version = utils.SplitNameVersion(args[0]) // TODO: version is not respected here, make a single file downloader per version with a different API URL.
			}

			for _, f := range files {
				if f.Name == opts.Snippet {
					f.Package = opts.Pkg
					// f.Data = TODO: pass from flags.
					f.Dest = opts.Dest
					return f.Install()
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&opts.Repo, "repo", opts.Repo, "--repo=iris-contrib/snippets")
	cmd.Flags().StringVar(&opts.Pkg, "pkg", opts.Pkg, "--pkg=mypackage")
	cmd.Flags().StringVar(&opts.Dest, "dest", opts.Dest, "--dest=./")

	return cmd
}
