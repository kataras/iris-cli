package cmd

import (
	"fmt"
	"sort"

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
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := reg.Load(); err != nil {
				return err
			}

			if len(args) == 0 {
				availableNames := make([]string, 0, len(reg.Projects))
				for name := range reg.Projects {
					availableNames = append(availableNames, name)
				}
				sort.Strings(availableNames)

				qs := []*survey.Question{
					{
						Name:   "name",
						Prompt: &survey.Select{Message: "Choose a project to install:", Options: availableNames, PageSize: 10},
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
				opts.Name = args[0]
			}

			if !reg.Exists(opts.Name) {
				return fmt.Errorf("project <%s> is not available", opts.Name)
			}

			if !utils.Exists(opts.Dest) {
				cmd.Printf("Directory <%s> will be created.\n", opts.Dest)
			}

			defer utils.ShowIndicator(cmd.OutOrStderr())()
			return reg.Install(opts.Name, opts.Module, opts.Dest)
		},
	}

	cmd.Flags().StringVar(&reg.Endpoint, "registry", reg.Endpoint, "--registry=URL or local file")
	cmd.Flags().StringVar(&opts.Dest, "dest", opts.Dest, "--dest=empty for current working directory or %GOPATH%/author")
	cmd.Flags().StringVar(&opts.Module, "module", opts.Module, "--module=local module name")

	return cmd
}
