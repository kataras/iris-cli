package cmd

import (
	"fmt"

	"github.com/kataras/iris-cli/project"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

func runCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "run",
		Short:         "Run starts a project",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "." // current directory.
			if len(args) > 0 {
				name = args[0]
			}

			p, err := project.LoadFromDisk(name)
			if err != nil {
				if err == project.ErrProjectNotExists {
					p, err = project.LoadFromDisk(".")
					if err == nil && p.Name == name {
						// argument is not a path but a project name which exists in the current dir.
						// ./myproject -> empty
						// iris-cli run svelte -> installs and builds the project
						// and again iris-cli run svelte -> it's not a directory but it's a project which meant to be ran.
					} else {
						doInstall := true
						err = survey.AskOne(&survey.Confirm{Message: fmt.Sprintf("%s does not exist, do you want to install it?", name), Default: doInstall}, &doInstall)
						if err != nil {
							return err
						}

						if doInstall {
							if err := RunCommand(cmd, "new", name /* , "--registry=./_testfiles/registry.yml" */); err != nil {
								return err
							}

							p, err = project.LoadFromDisk(".")
						} else {
							return nil
						}
					}

				}
			}

			if err != nil {
				return err
			}

			return p.Run(cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	return cmd
}
