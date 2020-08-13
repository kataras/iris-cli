package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kataras/iris-cli/utils"

	"github.com/spf13/cobra"
)

// stats --download-count [modules]
//       --versions [modules]
func statsCommand() *cobra.Command {
	var (
		showDownloadCount bool
		listVersions      bool
	)

	cmd := &cobra.Command{
		Use:           "stats --download-count <module1> <module2>",
		Short:         "fetch and calculate stats",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			modules := []string{"github.com/kataras/iris", "github.com/kataras/iris/v12"}
			if len(args) > 0 {
				modules = args
			}

			sort.Strings(modules)
			// TODO: JSON format?

			if showDownloadCount {
				return executeShowDownloadCount(cmd, modules)
			}
			if listVersions {
				return executeListVersions(cmd, modules)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&showDownloadCount, "download-count", showDownloadCount, "--download-count to fetch download count")
	cmd.Flags().BoolVar(&listVersions, "versions", listVersions, "--versions to list versions")

	return cmd
}

func executeShowDownloadCount(cmd *cobra.Command, modules []string) error {
	totalModuleDownloads := make(map[string]int)
	baseModule := ""
	for _, module := range modules {

		// if len(totalModuleDownloads) > 0 { to leave a new line after base end.
		// 	if _, ok := totalModuleDownloads[baseModule]; !ok {
		// 		cmd.Println()
		// 	}
		// }

		cmd.Printf("[%s]\n", module)
		total := 0
		for _, service := range goProxies {
			serviceName := service.name()
			count, err := service.getDownloadCount(module)
			if err != nil {
				if err == errNotImplemented {
					// cmd.Printf("[%s] service is missing a download count API\n", serviceName)
					// Let's just skip it.
					continue
				}
				return err
			}
			total += count
			cmd.Printf("• %s: %d\n", serviceName, count)
		}

		if total > 0 {
			cmd.Printf("• total: %d\n", total)
		}

		baseModule = baseModulePath(module)
		totalModuleDownloads[baseModule] += total
	}

	if len(totalModuleDownloads) > 0 {
		cmd.Println()
		cmd.Println("[repository total]")
		for _, module := range modules {
			for baseModule, total := range totalModuleDownloads {
				if baseModulePath(module) == baseModule {
					cmd.Printf("• %s: %d\n", baseModule, total)
					delete(totalModuleDownloads, baseModule)
				}
			}
		}
	}

	return nil
}

func executeListVersions(cmd *cobra.Command, modules []string) error {
	for _, module := range modules {
		cmd.Printf("[%s]\n", module)
		for _, service := range goProxies {
			serviceName := service.name()
			versions, err := service.listVersions(module)
			if err != nil {
				if err == errNotImplemented {
					// cmd.Printf("[%s] service is missing a list versions API\n", serviceName)
					// Let's just skip it.
					continue
				}
				return err
			}
			cmd.Printf("• %s:\n", serviceName)
			for _, version := range versions {
				cmd.Printf("  • %s\n", version)
			}
		}
	}

	return nil
}

func baseModulePath(baseModule string) string {
	n := len(baseModule) - 1
	isGopkg := strings.HasPrefix(baseModule, "gopkg.in")
	for i := n; i > -1; i-- {
		ch := baseModule[i]
		if isDigit(ch) || ch == 'v' {
			continue
		} else if ch == '/' || (isGopkg && ch == '.') {
			baseModule = baseModule[0:i]
			break
		} else {
			break
		}
	}

	return baseModule
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

var errNotImplemented = errors.New("not implemented")

var goProxies = []goProxy{ // order.
	newGoProxyCN(),
	newGoCenterIO(),
	newGoProxyIO(),
}

type goProxy interface {
	name() string
	// Example: https://goproxy.cn/stats/github.com/kataras/iris/v12
	getDownloadCount(module string) (int, error)

	// Example:
	// https://goproxy.io/github.com/kataras/iris/@v/list
	// https://goproxy.io/github.com/kataras/iris/v12/@v/list
	listVersions(module string) ([]string, error)
}

type (
	moduleInfo struct {
		repo string // e.g. github.com/kataras/iris
		// by version we mean the suffix of the module here, no v12.1.8 and e.t.c
		versions []versionInfo // e.g. v11, v12
	}

	versionInfo struct {
		downloadCount int
	}
)

type goProxyCN struct{}

var _ goProxy = (*goProxyCN)(nil)

func newGoProxyCN() *goProxyCN {
	return &goProxyCN{}
}

func (p *goProxyCN) name() string {
	return "goproxy.cn"
}

func (p *goProxyCN) url(format string, args ...interface{}) string {
	return fmt.Sprintf("https://goproxy.cn%s", fmt.Sprintf(format, args...))
}

func (p *goProxyCN) getDownloadCount(module string) (int, error) {
	url := p.url("/stats/%s", module)

	var stats = struct {
		DownloadCount int `json:"download_count"`
		Last30Days    []struct {
			Date          time.Time `json:"date"`
			DownloadCount int       `json:"download_count"`
		} `json:"last_30_days"`
		Top10ModuleVersions []struct {
			DownloadCount int    `json:"download_count"`
			ModuleVersion string `json:"module_version"`
		} `json:"top_10_module_versions"`
	}{}

	if err := utils.ReadJSON(url, &stats); err != nil {
		return 0, err
	}

	return stats.DownloadCount, nil
}

func (p *goProxyCN) listVersions(module string) ([]string, error) {
	return nil, errNotImplemented
}

type goCenterIO struct{}

var _ goProxy = (*goCenterIO)(nil)

func newGoCenterIO() *goCenterIO {
	return &goCenterIO{}
}

func (p *goCenterIO) name() string {
	return "gocenter.io"
}

func (p *goCenterIO) getDownloadCount(module string) (int, error) {
	base := baseModulePath(module)
	url := fmt.Sprintf("https://search.gocenter.io/api/ui/search?name_fragment=%s", base)

	var stats = struct {
		Count   int `json:"count"`
		Modules []struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			Downloads     int    `json:"downloads"`
			Stars         int    `json:"stars"`
			LatestVersion string `json:"latest_version"`
		} `json:"modules"`
	}{}

	if err := utils.ReadJSON(url, &stats); err != nil {
		return 0, err
	}

	for _, moduleStat := range stats.Modules {
		if moduleStat.Name == module {
			return moduleStat.Downloads, nil
		}
	}

	return 0, nil
}

func (p *goCenterIO) listVersions(module string) ([]string, error) {
	return nil, errNotImplemented
}

type goProxyIO struct{}

var _ goProxy = (*goProxyIO)(nil)

func newGoProxyIO() *goProxyIO {
	return &goProxyIO{}
}

func (p *goProxyIO) name() string {
	return "goproxy.io"
}

func (p *goProxyIO) getDownloadCount(module string) (int, error) {
	return 0, errNotImplemented
}

func (p *goProxyIO) listVersions(module string) ([]string, error) {
	url := fmt.Sprintf("https://goproxy.io/%s/@v/list", module)
	b, err := utils.Download(url, nil)
	if err != nil {
		return nil, err
	}

	versions := strings.Split(string(b), "\n")
	switch n := len(versions); n {
	case 0: // this should never happen.
		return nil, nil
	case 1:
		if versions[0] == "" { // check if contains at least one version.
			return nil, nil
		}
		fallthrough
	default:
		if versions[n-1] == "" { // remove last \n.
			versions = versions[0 : n-1]
		}
	}

	return versions, nil
}
