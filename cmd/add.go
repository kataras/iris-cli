package cmd

import (
	"encoding/json"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/kataras/iris-cli/snippet"
	"github.com/kataras/iris-cli/utils"

	"github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	"github.com/spf13/cobra"
)

const defaultRepo = "iris-contrib/snippets"

// iris-cli add --repo=iris-contrib/snippets logger.go@v0.0.10
// iris-cli add logger.go
// iris-cli add --repo=iris-contrib/snippets
// iris-cli add --data=data.json --repo=kataras/iris _examples/view/template_html_0/templates/hi.html
func addCommand() *cobra.Command {
	var (
		file = snippet.File{
			Repo:    defaultRepo,
			Version: "master",
			Dest:    "./",
			Package: "",
		}

		tmplDataFile string // TODO: read JSON with prompt and flag.
		readDataFile = func(path string, ptr interface{}) {
			b, _ := ioutil.ReadFile(path)
			json.Unmarshal(b, &ptr)
		}
	)

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
						Prompt: &survey.Select{Message: "Select snippet:", Options: availableSnippets, Default: file.Name, PageSize: 15},
					},
					{
						Name: "Package",
						Prompt: &survey.Input{Message: "What should be the new package name?", Default: file.Package,
							Help: "Leave it empty to be resolved automatically by your current project's files"},
					},
					{
						Name:     "_Data",
						Validate: nil,
						Transform: func(ans interface{}) (newAns interface{}) {
							if path, ok := ans.(string); ok {
								readDataFile(path, &file.Data) // can't set as &newAns because survey is uncorrectly passes is as string without checks on its input.go#101,
							}
							return
						},

						Prompt: &survey.Input{Message: "Load template data from json file:", Default: tmplDataFile,
							Help: "Leave it empty if the snippet is not a template"},
					},
					{
						Name:   "Dest",
						Prompt: &survey.Input{Message: "Choose a file or directory to be saved:", Default: file.Dest},
					},
				}

				if err := survey.Ask(qs, &file); err != nil {
					if s, ok := surveycore.IsFieldNotMatch(err); ok {
						if s[0] == '_' {
							err = nil
						}
					}

					if err != nil {
						return err
					}
				}
			} else {
				file.Name, file.Version = utils.SplitNameVersion(args[0])

				if tmplDataFile != "" {
					readDataFile(tmplDataFile, &file.Data)
				}
			}

			return file.Install()
		},
	}

	cmd.Flags().StringVar(&file.Repo, "repo", file.Repo, "--repo=iris-contrib/snippets")
	cmd.Flags().StringVar(&file.Package, "pkg", file.Package, "--pkg=mypackage")
	cmd.Flags().StringVar(&file.Dest, "dest", file.Dest, "--dest=./")
	cmd.Flags().StringToStringVar(&file.Replacements, "replace", nil, "--replace=oldValue=newValue,oldValue2=newValue2")
	cmd.Flags().StringVar(&tmplDataFile, "data", "", "--data=data.json")

	return cmd
}
