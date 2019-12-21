package cmd

import (
	"github.com/spf13/cobra"
)

// New returns the root command.
func New(buildRevision, buildTime string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "iris-cli",
		Short: "Command Line Interface for Iris",
		Long: `Iris CLI is a tool for Iris Web Framework.
It can be used to install starter kits and project structures 
Complete documentation is available at https://github.com/kataras/iris-cli`,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	helpTemplate := HelpTemplate{
		BuildRevision:        buildRevision,
		BuildTime:            buildTime,
		ShowGoRuntimeVersion: true,
	}
	rootCmd.SetHelpTemplate(helpTemplate.String())

	// Commands.
	rootCmd.AddCommand(newCommand())

	return rootCmd
}
