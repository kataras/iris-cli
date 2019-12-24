package cmd

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/kataras/iris-cli/project"
	"github.com/kataras/iris-cli/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
)

// iris-cli new --registry=./_testfiles/registry.json
// iris-cli new --registry=./_testfiles/registry.json --dest=%GOPATH%/github.com/author --module=github.com/author/neffos github.com/kataras/neffos@master
func newCommand() *cobra.Command {
	var (
		reg = project.NewRegistry()

		opts = project.Project{
			Version: "master",
			Dest:    "./",
			Reader: func(r io.Reader) ([]byte, error) {
				tmpl := `{{etime . "%s elapsed"}} {{speed . }}`
				//									Content-Length is not available
				//									on Github release download response.
				bar := pb.ProgressBarTemplate(tmpl).Start64(0).SetMaxWidth(45)
				defer bar.Finish()

				b, err := ioutil.ReadAll(bar.NewProxyReader(r))
				bar.SetTemplateString(`{{etime . "%s elapsed"}} [{{string . "all_bytes" | green}}]`)
				bar.Set("all_bytes", formatByteLength(len(b)))
				return b, err
			},
		}
	)

	cmd := &cobra.Command{
		Use:           "new",
		Short:         "New creates a new starter kit project.",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Printf("Loading projects from <%s>\n", reg.Endpoint)
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
				if len(availableVersions) == 0 {
					availableVersions = []string{"latest"}
				} else if len(availableVersions) > 1 {
					availableVersions[0] = availableVersions[0] + " (latest)"
				}
				qs := []*survey.Question{
					{
						Name:   "version",
						Prompt: &survey.Select{Message: "Select version:", Options: availableVersions, Default: opts.Version, PageSize: 5},
					},
					{
						Name: "module",
						Prompt: &survey.Input{Message: "What should be the new module name?", Default: opts.Module,
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
				opts.Name, opts.Version = utils.SplitNameVersion(args[0]) // split by @.
			}

			if !utils.Exists(opts.Dest) {
				cmd.Printf("Directory <%s> will be created.\n", opts.Dest)
			}

			err := reg.Install(&opts)
			if err != nil {
				return err
			}

			// cmd.Printf("Project <%s> created.\n", opts.Dest)
			return nil
		},
	}

	cmd.Flags().StringVar(&reg.Endpoint, "registry", reg.Endpoint, "--registry=URL or local file")
	cmd.Flags().StringVar(&opts.Dest, "dest", opts.Dest, "--dest=empty for current working directory or %GOPATH%/author")
	cmd.Flags().StringVar(&opts.Module, "module", opts.Module, "--module=local module name")

	return cmd
}

func formatByteLength(b int) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}
