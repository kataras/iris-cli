package cmd

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/kataras/iris-cli/utils"

	"github.com/spf13/cobra"
)

// stats compare --download-count --since=24h10m5s --src=downloads.yml
func statsCompareCommand() *cobra.Command {
	var (
		compareDownloadCount = true
		pretty               = true
		src                  string
		since                string
	)

	cmd := &cobra.Command{
		Use:           "compare --download-count --src=downloads.yml [<module1> <module2>]",
		Short:         "stats compare",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var history []stats
			if err := utils.Import(src, &history); err != nil {
				return err
			}
			sort.Slice(history, func(i, j int) bool {
				return history[i].Timestamp < history[j].Timestamp
			})

			var sinceTimestamp int64
			if since != "" {
				switch since {
				case "yesterday":
					sinceTimestamp = time.Now().Add(-24 * time.Hour).Unix()
				case "now":
					// sinceTimestamp = time.Now().Unix()
					// calculateDownloadCount(...)
					// export... re run
					fallthrough
				default:
					if strings.Contains(since, "-") {
						// try parse by specific datetime.
						t, err := time.Parse(timeFormat, since)
						if err != nil {
							return fmt.Errorf("since parse datetime [%s]: %v", since, err)
						}

						sinceTimestamp = t.UTC().Unix()
					} else {
						// try parse duration.
						d, err := time.ParseDuration(since)
						if err != nil {
							return fmt.Errorf("since parse duration [%s]: %v", since, err)
						}

						sinceTimestamp = time.Now().Add(-d).UTC().Unix()
					}

				}
			}

			_ = sinceTimestamp

			var comparedHistory []stats
			for _, h := range history {
				if len(h.TotalDownloadCounts) == 0 {
					continue
				}
				if sinceTimestamp > h.Timestamp {
					continue
				}

				comparedHistory = append(comparedHistory, h)
				// cmd.Printf("%#+v\n", h)
			}

			for _, h := range comparedHistory {
				t := time.Unix(h.Timestamp, 0)
				timeFormatted := ""
				if pretty {
					timeFormatted = humanize.Time(t)
				} else {
					if timeFormat == "" {
						timeFormat = http.TimeFormat
					}
					timeFormatted = t.Format(timeFormat)
				}

				cmd.Printf("[%s]\n", timeFormatted)
				for _, counts := range h.TotalDownloadCounts {
					cmd.Printf("  • %s: %d\n", counts.Module, counts.TotalDownloadCount)
				}
			}

			if n := len(comparedHistory); n >= 2 {
				first := comparedHistory[0]
				last := comparedHistory[n-1]

				cmd.Println()
				cmd.Println("[diff]")

				for i, counts := range last.TotalDownloadCounts {
					newDownloadCount := counts.TotalDownloadCount - first.TotalDownloadCounts[i].TotalDownloadCount
					cmd.Printf("  • %s: +%d\n", counts.Module, newDownloadCount)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&compareDownloadCount, "download-count", compareDownloadCount, "--download-count to compare downloads")
	cmd.Flags().BoolVar(&pretty, "pretty", pretty, "--pretty=false to disable humanize time")
	cmd.Flags().StringVar(&src, "src", src, "--src=downloads.yml to import history data from previous stats commands")
	cmd.Flags().StringVar(&since, "since", since, "--since=1h a duration, or specific datetime to filter based on early date")

	cmd.MarkFlagRequired("src")
	return cmd
}
