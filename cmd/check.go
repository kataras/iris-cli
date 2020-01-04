package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/kataras/iris-cli/parser"
	"github.com/kataras/iris-cli/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

const irisRepo = "kataras/iris"

// iris-cli check
// iris-cli check gopkg.in/yaml.v2
// iris-cli check all
func checkCommand() *cobra.Command { // maintenance
	var (
		forceUpdate bool
	)

	cmd := &cobra.Command{
		Use:           "check",
		Short:         "Check performs maintenance, if major, patch or minor update is available after confirmation, installs the latest Iris version or 'all'.",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			modulePath := irisRepo
			if len(args) > 0 {
				// if we have more features in the future
				// then they should move to subcommands and if not subcommand with this name found then perform update checks.
				modulePath = args[0]
			}

			if modulePath == "iris" || modulePath == irisRepo {
				// grab the latest one by its remote go.mod (e.g. /v12 to /v13)
				modulePath, err = getModulePath(irisRepo)
				if err != nil {
					return
				}
			} // it can be "all".

			goList := utils.Command("go", "list", "-u", "-m", "-json", modulePath)
			out, err := goList.Output()
			if err != nil {
				return fmt.Errorf("module <%s> not found", modulePath)
			}

			totalModulesLen := 0
			var outdatedModules []module

			dec := json.NewDecoder(bytes.NewReader(out))
			for {
				var m module

				err := dec.Decode(&m)
				if err != nil {
					// goList.Wait()
					if err == io.EOF {
						break
					}

					return err
				}
				totalModulesLen++

				if m.Update != nil {
					outdatedModules = append(outdatedModules, m)
				}
			}

			updateModulesNameVersion := make([]string, len(outdatedModules))
			for i := range outdatedModules {
				m := outdatedModules[i]
				updateModulesNameVersion[i] = fmt.Sprintf("%s %s => %s", m.Path, m.Version, m.Update.Version)
			}

			if len(outdatedModules) > 0 {
				survey.MultiSelectQuestionTemplate = `
{{- if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }} {{ .Help }}{{color "reset"}}{{"\n"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{ .Config.Icons.Question.Text }} {{color "reset"}}
{{- color "default+hb"}}{{ .Message }}{{ .FilterMessage }}{{color "reset"}}
{{- if .ShowAnswer}}{{color "cyan"}} {{.Answer}}{{color "reset"}}{{"\n"}}
{{- else }}
	{{- "  "}}{{- color "cyan"}}[Use arrows to move, space to uncheck and enter to select, type to filter{{- if and .Help (not .ShowHelp)}}, {{ .Config.HelpInput }} for more help{{end}}]{{color "reset"}}
  {{- "\n"}}
  {{- range $ix, $option := .PageEntries}}
    {{- if eq $ix $.SelectedIndex }}{{color $.Config.Icons.SelectFocus.Format }}{{ $.Config.Icons.SelectFocus.Text }}{{color "reset"}}{{else}} {{end}}
    {{- if index $.Checked $option.Index }}{{color $.Config.Icons.MarkedOption.Format }} {{ $.Config.Icons.MarkedOption.Text }} {{else}}{{color $.Config.Icons.UnmarkedOption.Format }} {{ $.Config.Icons.UnmarkedOption.Text }} {{end}}
    {{- color "reset"}}
    {{- " "}}{{$option.Value}}{{"\n"}}
  {{- end}}
{{- end}}`
				var selectedUpdateModulesNameVersion []string

				if !forceUpdate {
					err := survey.AskOne(&survey.MultiSelect{
						Message:  fmt.Sprintf("%d/%d modules are outdated, select to update", len(outdatedModules), totalModulesLen),
						PageSize: 10,
						Options:  updateModulesNameVersion,
						Default:  updateModulesNameVersion,
					}, &selectedUpdateModulesNameVersion)
					if err != nil {
						return err
					}
				} else {
					selectedUpdateModulesNameVersion = updateModulesNameVersion
				}

				if len(selectedUpdateModulesNameVersion) == 0 {
					return nil
				}

				succeedLen := 0
				var failedToUpdateNameVersion []string
				for _, nameVersion := range selectedUpdateModulesNameVersion {
					for _, m := range outdatedModules {
						if m.String() == nameVersion {
							goGet := utils.Command("go", "get", "-u", m.Path+"@"+m.Update.Version)
							out, gErr := goGet.CombinedOutput()
							if gErr != nil {
								failedToUpdateNameVersion = append(failedToUpdateNameVersion, nameVersion)
								errStr := fmt.Sprintf("[%d] %s:\n%v", len(failedToUpdateNameVersion), nameVersion, string(out))
								if err != nil {
									err = fmt.Errorf("%v\n%s", err, errStr)
								} else {
									err = fmt.Errorf(errStr)
								}

								continue // do not fail.
							}

							succeedLen++
						}
					}
				}

				//	cmd.Printf("%d modules failed to update:\n%s\n", len(failedToUpdateNameVersion), strings.Join(failedToUpdateNameVersion, "\n"))
				cmd.Printf("%d modules are updated successfully.\n", succeedLen)
				if err != nil {
					cmd.Printf("Failed to update %d modules:\n", len(selectedUpdateModulesNameVersion)-succeedLen)
					return err
				}
			} else {
				cmd.Printf("%d modules are up-to-date.\n", totalModulesLen)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&forceUpdate, "force-update", forceUpdate, "--force-update to update any outdated modules without confirmation")

	return cmd
}

type (
	module struct {
		Path      string       // module path
		Version   string       // module version
		Versions  []string     // available module versions (with -versions)
		Replace   *module      // replaced by this module
		Time      *time.Time   // time version was created
		Update    *module      // available update, if any (with -u)
		Main      bool         // is this the main module?
		Indirect  bool         // is this module only an indirect dependency of main module?
		Dir       string       // directory holding files for this module, if any
		GoMod     string       // path to go.mod file for this module, if any
		GoVersion string       // go version used in module
		Error     *moduleError // error loading module
	}

	moduleError struct {
		Err string // the error itself
	}
)

func (m module) String() string {
	return fmt.Sprintf("%s %s => %s", m.Path, m.Version, m.Update.Version)
}

// getModulePath returns the module path of a github repository.
func getModulePath(repo string) (string, error) {
	b, err := utils.DownloadFile(repo, "", "go.mod")
	if err != nil {
		return "", err
	}

	modulePath := parser.ModulePath(b)
	if len(modulePath) == 0 {
		return "", fmt.Errorf("module path: %s: unable to parse remote go.mod", repo)
	}

	return string(modulePath), nil
}
