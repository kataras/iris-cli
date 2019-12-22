package cmd

import (
	"fmt"

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
			// arguments.
			Name string
			// flags.
			Version string
			Module  string
			Dest    string
		}{
			Version: "master",
			Dest:    "./",
		}
	)

	cmd := &cobra.Command{
		Use:           "new",
		Short:         "New creates a new starter kit project.",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := reg.Load(); err != nil {
				return err
			}

			if len(args) == 0 {

				err := survey.AskOne(&survey.Select{Message: "Choose a project to install:", Options: reg.Names, PageSize: 10}, &opts.Name)
				if err != nil {
					return err
				}

				repo, ok := reg.Exists(opts.Name)
				if !ok {
					return fmt.Errorf("project <%s> is not available", opts.Name)
				}

				availableVersions := utils.ListReleases(repo)
				availableVersions = append([]string{"latest"}, availableVersions...)
				qs := []*survey.Question{
					{
						Name:   "version",
						Prompt: &survey.Select{Message: "Select version:", Options: availableVersions, Default: opts.Version, PageSize: 5},
					},
					{
						Name: "module",
						Prompt: &survey.Input{Message: "Type the Go module name:", Default: opts.Module,
							Help: "Leave it empty to be the same as the remote repository or type a different go module name for your project"},
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
				opts.Name, opts.Version = project.SplitName(args[0]) // split by @.
			}

			if !utils.Exists(opts.Dest) {
				cmd.Printf("Directory <%s> will be created.\n", opts.Dest)
			}

			defer utils.ShowIndicator(cmd.OutOrStderr())()
			return reg.Install(opts.Name, opts.Version, opts.Module, opts.Dest)
		},
	}

	cmd.Flags().StringVar(&reg.Endpoint, "registry", reg.Endpoint, "--registry=URL or local file")
	cmd.Flags().StringVar(&opts.Dest, "dest", opts.Dest, "--dest=empty for current working directory or %GOPATH%/author")
	cmd.Flags().StringVar(&opts.Module, "module", opts.Module, "--module=local module name")

	return cmd
}
