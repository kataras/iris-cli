package cmd

import (
	"github.com/kataras/iris-cli/project"
	"strings"

	"github.com/spf13/cobra"
)

// iris-cli new --registry=./_testfiles/registry.json myproject.com@v12
func newCommand() *cobra.Command {
	var (
		reg = project.NewRegistry()

		dest = "./"
	)

	cmd := &cobra.Command{
		Use:           "new",
		Short:         "New creates a new starter kit project.",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// TODO: make interactive selection of the available projects retrieved from the registry.
				for idx, p := range reg.Projects {
					cmd.Printf("[%d] [%s] %s\n", idx, p.Branch, p.Repo)
				}
				return nil
			}

			// TODO: install section.
			repoBranch := strings.Split(args[0], "@") // e.g. github.com/author/project@v12
			p := project.New(dest, repoBranch[0])
			if len(repoBranch) > 1 {
				p.Branch = repoBranch[1]
			}

			cmd.Printf("%#+v\n", p)

			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error { return reg.Load() },
	}

	cmd.Flags().StringVar(&reg.Endpoint, "registry", reg.Endpoint, "--registry=URL or local file")
	cmd.Flags().StringVar(&dest, dest, "./", "--dest=empty for %GOPATH if available or current working directory")

	return cmd
}
