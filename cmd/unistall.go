package cmd

import (
	"path/filepath"

	"github.com/kataras/iris-cli/project"
	"github.com/kataras/iris-cli/utils"

	"github.com/spf13/cobra"
)

func unistallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "unistall",
		Short:         "Removes all project files",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "." // current directory.
			if len(args) > 0 {
				name = args[0]
			}

			projectPath, err := filepath.Abs(name)
			if err != nil {
				return err
			}

			if !utils.Exists(projectPath) {
				return project.ErrProjectNotExists
			}

			p, err := project.LoadFromDisk(projectPath)
			if err != nil {
				return err
			}

			return p.Unistall()
		},
	}

	return cmd
}
