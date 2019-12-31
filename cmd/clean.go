package cmd

import (
	"fmt"
	"github.com/kataras/iris-cli/project"
	"path/filepath"

	"github.com/kataras/iris-cli/utils"

	"github.com/spf13/cobra"
)

// TODO:
func cleanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "clean",
		Short:         "Clean a project after install or build.",
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
				return fmt.Errorf("project does not exist")
			}

			p, err := project.LoadFromDisk(projectPath)
			if err != nil {
				return err
			}

			cmd.Printf("%#+v\n", p)

			return nil
		},
	}

	return cmd
}
