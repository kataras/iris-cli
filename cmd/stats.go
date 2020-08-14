package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kataras/iris-cli/utils"

	"github.com/kataras/golog"
	"github.com/spf13/cobra"
)

// stats --download-count [modules]
//       --versions [modules]
//
// go run -race main.go stats -v --download-count --out=downloads.yml gopkg.in/yaml.v2 gopkg.in/yaml.v3 github.com/kataras/iris github.com/kataras/iris/v12
func statsCommand() *cobra.Command {
	var (
		showDownloadCount bool
		listVersions      bool
		out               string
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

			// TODO: JSON format?

			if showDownloadCount {
				if err := executeShowDownloadCount(cmd, modules, out); err != nil {
					return err
				}
			}
			if listVersions {
				if err := executeListVersions(cmd, modules, showDownloadCount); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&out, "out", out, "--out=downloads.yml to export total download counts")
	cmd.Flags().BoolVar(&showDownloadCount, "download-count", showDownloadCount, "--download-count to fetch download count")
	cmd.Flags().BoolVar(&listVersions, "versions", listVersions, "--versions to list versions")

	cmd.AddCommand(statsCompareCommand())

	return cmd
}

func executeShowDownloadCount(cmd *cobra.Command, modules []string, output string) error {
	st, err := calculateDownloadCount(modules)
	if err != nil {
		return err
	}

	for _, downloadCount := range st.DownloadCounts {
		cmd.Printf("[%s]\n", downloadCount.Module)

		for serviceName, count := range downloadCount.DownloadCount {
			cmd.Printf("• %s: %d\n", serviceName, count)
		}

		if total := downloadCount.TotalDownloadCount; total > 0 {
			cmd.Printf("• total: %d\n", total)
		}
	}

	if len(st.TotalDownloadCounts) > 0 {
		cmd.Println()
		cmd.Println("[repository total]")

		for _, downloadCount := range st.TotalDownloadCounts {
			if total := downloadCount.TotalDownloadCount; total > 0 {
				cmd.Printf("• %s: %d\n", downloadCount.Module, downloadCount.TotalDownloadCount)
			}
		}

		if output != "" {
			err := exportStats([]stats{st}, output)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

type stats struct {
	Timestamp           int64                 `json:"timestamp" yaml:"Timestamp"`
	TotalDownloadCounts []*downloadCountStats `json:"total_download_counts" yaml:"TotalDownloadCounts"` // base module's total download count.
	DownloadCounts      []*downloadCountStats `json:"-" yaml:"-"`
}

type downloadCountStats struct {
	Module             string           `json:"module" yaml:"Module"`
	DownloadCount      map[string]int64 `json:"-" yaml:"-"` // service name download count.
	TotalDownloadCount int64            `json:"download_count" yaml:"DownloadCount"`
}

func calculateDownloadCount(modules []string) (stats, error) { // TODO: prettify it, it works and it has no race conditions but it should be clean.
	st := stats{
		Timestamp: time.Now().Unix(),
	}

	totalDownloadStats := make(map[string]*downloadCountStats)
	mu := new(sync.RWMutex)

	calc := func(module string) error {
		var total int64

		baseModule := baseModulePath(module)

		dlStats := &downloadCountStats{
			Module:        module,
			DownloadCount: make(map[string]int64),
		}

		calcProxy := func(service goProxy) error {
			serviceName := service.name()
			count, err := service.getDownloadCount(module)
			if err != nil {
				if err == errNotImplemented {
					// cmd.Printf("[%s] service is missing a download count API\n", serviceName)
					// Let's just skip it.
					return nil
				}
				return err
			}

			if count == 0 {
				return nil
			}

			mu.Lock()
			dlStats.DownloadCount[serviceName] = count
			dlStats.TotalDownloadCount += count
			total += count
			mu.Unlock()

			return nil
		}

		mu.RLock()
		dlTotalStats, hasPrevBaseModule := totalDownloadStats[baseModule]
		mu.RUnlock()
		if dlTotalStats == nil {
			dlTotalStats = &downloadCountStats{
				Module:        baseModule,
				DownloadCount: make(map[string]int64),
			}
			mu.Lock()
			totalDownloadStats[baseModule] = dlTotalStats
			mu.Unlock()
		}

		wgProxies := new(sync.WaitGroup)
		wgProxies.Add(len(goProxies))
		var err error
		errLock := new(sync.Mutex)

		for _, service := range goProxies {
			go func(service goProxy) {
				defer wgProxies.Done()

				calcErr := calcProxy(service)
				if calcErr != nil {
					errLock.Lock()
					if err == nil {
						err = errors.New("")
					} else {
						err = fmt.Errorf("%v, %v", err, calcErr)
					}
					errLock.Unlock()
				}
			}(service)
		}

		wgProxies.Wait()

		mu.Lock()
		st.DownloadCounts = append(st.DownloadCounts, dlStats)
		mu.Unlock()

		if total > 0 {
			mu.Lock()
			dlTotalStats.TotalDownloadCount += total
			mu.Unlock()
			if !hasPrevBaseModule {
				mu.Lock()
				st.TotalDownloadCounts = append(st.TotalDownloadCounts, dlTotalStats)
				mu.Unlock()
			}
		}

		return err
	}

	var err error
	errLock := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	wg.Add(len(modules))
	start := time.Now()
	for _, module := range modules {
		go func(module string) { // we try to collect all errors, we do not abort on first error.
			defer wg.Done()
			calcErr := calc(module)
			if calcErr != nil {
				errLock.Lock()
				if err == nil {
					err = errors.New("")
				} else {
					err = fmt.Errorf("%v, %v", err, calcErr)
				}
				errLock.Unlock()
			}
		}(module)
	}

	wg.Wait()
	golog.Debugf("Time elapsed to calculate download stats: %s", time.Since(start))

	st.DownloadCounts = sortDownloadCountStats(st.DownloadCounts)
	st.TotalDownloadCounts = sortDownloadCountStats(st.TotalDownloadCounts)
	return st, err
}

func sortDownloadCountStats(dlStats []*downloadCountStats) []*downloadCountStats {
	sort.Slice(dlStats, func(i, j int) bool {
		return dlStats[i].Module < dlStats[j].Module
	})

	return dlStats
}

func exportStats(st []stats, inout string) error {
	var temp []stats
	if err := utils.Import(inout, &temp); err != nil {
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, os.ErrPermission) { // else we don't care, we will create it.
			return err
		}
	}

	st = append(temp, st...)

	// for _, stat := range st {
	// 	stat.DownloadCounts = sortDownloadCountStats(stat.DownloadCounts)
	// 	stat.TotalDownloadCounts = sortDownloadCountStats(stat.TotalDownloadCounts)
	// }

	return utils.Export(inout, st)
}

func executeListVersions(cmd *cobra.Command, modules []string, prependNewLine bool) error {
	if prependNewLine {
		cmd.Println()
	}
	sort.Strings(modules)

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
	getDownloadCount(module string) (int64, error)

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
		downloadCount int64
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

func (p *goProxyCN) getDownloadCount(module string) (int64, error) {
	url := p.url("/stats/%s", module)

	var stats = struct {
		DownloadCount int64 `json:"download_count"`
		Last30Days    []struct {
			Date          time.Time `json:"date"`
			DownloadCount int64     `json:"download_count"`
		} `json:"last_30_days"`
		Top10ModuleVersions []struct {
			DownloadCount int64  `json:"download_count"`
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

func (p *goCenterIO) getDownloadCount(module string) (int64, error) {
	url := fmt.Sprintf("https://search.gocenter.io/api/ui/search?name_fragment=%s", module)

	var stats = struct {
		Count   int `json:"count"`
		Modules []struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			Downloads     int64  `json:"downloads"`
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

func (p *goProxyIO) getDownloadCount(module string) (int64, error) {
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
