package cmd

import (
	"github.com/spf13/cobra"
)

func newCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "New creates a new starter kit project",
		Run: func(cmd *cobra.Command, args []string) {
			println(args[0])
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// TODO: load known projects through Registry.
			return nil
		},
	}

	return cmd
}
