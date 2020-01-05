package cmd

import (
	"github.com/spf13/cobra"
)

func initCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "init",
		Short:         "Init creates the iris project file from a LOCAL git repository. Useful for custom projects",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: create .iris project, repository and other information may be fetched from git repository (through .git folder).
			return nil
		},
	}
	return cmd
}
