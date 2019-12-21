package cmd

import (
	"github.com/kataras/iris-cli/project"
	"github.com/kataras/iris-cli/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// iris-cli new --registry=./_testfiles/registry.json
// iris-cli new --registry=./_testfiles/registry.json --dest=%GOPATH%/github.com/author --module=github.com/author/neffos github.com/kataras/neffos@master
func newCommand() *cobra.Command {
	var (
		reg = project.NewRegistry()

		opts = struct {
			Repo   string
			Module string
			Dest   string
		}{
			Dest: "./",
		}
	)

	cmd := &cobra.Command{
		Use:           "new",
		Short:         "New creates a new starter kit project.",
		SilenceErrors: true,
		PreRunE:       func(cmd *cobra.Command, args []string) error { return reg.Load() },
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				availableRepos := make([]string, len(reg.Projects))
				for idx, p := range reg.Projects {
					availableRepos[idx] = p.Repo
				}

				qs := []*survey.Question{
					{
						Name:   "repo",
						Prompt: &survey.Select{Message: "Choose a repository:", Options: availableRepos, PageSize: 10},
					},
					{
						Name: "module",
						Prompt: &survey.Input{Message: "Go module:", Default: opts.Module,
							Help: "Leave it empty to be the same as the remote repository or choose a go module name for your project"},
					},
					{
						Name:   "dest",
						Prompt: &survey.Input{Message: "Choose directory to be installed:", Default: opts.Dest},
					},
				}

				if err := survey.Ask(qs, &opts); err != nil {
					return err
				}

			} else {
				opts.Repo = args[0]
			}

			if !utils.Exists(opts.Dest) {
				cmd.Printf("Directory <%s> will be created.\n", opts.Dest)
			}

			p := project.New(opts.Dest, opts.Repo)
			p.Module = opts.Module
			defer utils.ShowIndicator(cmd.OutOrStderr())()
			return p.Install()
		},
	}

	cmd.Flags().StringVar(&reg.Endpoint, "registry", reg.Endpoint, "--registry=URL or local file")
	cmd.Flags().StringVar(&opts.Dest, "dest", opts.Dest, "--dest=empty for current working directory or %GOPATH%/author")
	cmd.Flags().StringVar(&opts.Module, "module", opts.Module, "--module=local module name")

	return cmd
}
