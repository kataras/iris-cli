package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/kataras/iris-cli/project"
	"github.com/kataras/iris-cli/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

func runCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "run",
		Short:         "Run starts a project.",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("argument path to run is required")
			}

			name := args[0]
			projectPath, err := filepath.Abs(name)
			if err != nil {
				return err
			}

			if !utils.Exists(projectPath) {
				doInstall := false
				err := survey.AskOne(&survey.Confirm{Message: fmt.Sprintf("%s does not exist, do you want to install it?", name), Default: true}, &doInstall)
				if err != nil {
					return err
				}

				if doInstall {
					if err := RunCommand(cmd, "new", name /* , "--registry=./_testfiles/registry.json" */); err != nil {
						return err
					}
				}
			}

			return project.Run(projectPath, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	return cmd
}
