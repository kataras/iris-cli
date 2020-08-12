package cmd

import (
	"errors"
	"sort"

	"github.com/spf13/cobra"
)

// iris-cli stats --download-count github.com/kataras/iris
func statsCommand() *cobra.Command {
	var showDownloadCount bool

	cmd := &cobra.Command{
		Use:           "stats --download-count <module>",
		Short:         "fetch and calculate stats",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			modules := []string{"github.com/kataras/iris", "github.com/kataras/iris/v12"}
			if len(args) > 0 {
				modules = args
			}

			sort.Strings(modules)

			if showDownloadCount {
				// lastModule := ""
				for _, module := range modules {
					cmd.Println(module)
					for serviceName, service := range goProxies {
						count, err := service.getDownloadCount(module)
						if err != nil {
							if err == errNotImplemented {
								cmd.Printf("[%s] service is missing a download count API\n", serviceName)
								continue
							}
							return err
						}

						cmd.Printf("[%s] %d\n", serviceName, count)
					}
					cmd.Println()

					// lastModule = module
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&showDownloadCount, "download-count", showDownloadCount, "--download-count to fetch download count")

	return cmd
}

var errNotImplemented = errors.New("not implemented")

var goProxies = map[string]goProxy{
	"goproxy.cn": newGoProxyCN(),
	"goproxy.io": newGoProxyIO(),
}

type goProxy interface {
	// Example: https://goproxy.cn/stats/github.com/kataras/iris/v12
	getDownloadCount(module string) (int, error)

	// Example:
	// https://goproxy.io/github.com/kataras/iris/@v/list
	// https://goproxy.io/github.com/kataras/iris/v12/@v/list
	listVersions(module string) ([]string, error)
}

type goProxyCN struct{}

var _ goProxy = (*goProxyCN)(nil)

func newGoProxyCN() *goProxyCN {
	return &goProxyCN{}
}

func (p *goProxyCN) getDownloadCount(module string) (int, error) {

	return 0, nil
}

func (p *goProxyCN) listVersions(module string) ([]string, error) {
	return nil, errNotImplemented
}

type goProxyIO struct{}

var _ goProxy = (*goProxyIO)(nil)

func newGoProxyIO() *goProxyIO {
	return &goProxyIO{}
}

func (p *goProxyIO) getDownloadCount(module string) (int, error) {
	return 0, errNotImplemented
}

func (p *goProxyIO) listVersions(module string) ([]string, error) {
	return nil, nil
}
