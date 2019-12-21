package cmd

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// HelpTemplate is the structure which contaisn the variables for the help command.
type HelpTemplate struct {
	BuildTime            string
	BuildRevision        string
	ShowGoRuntimeVersion bool

	Template fmt.Stringer
}

func (h HelpTemplate) String() string {
	buildTitle := ">>>> build" // if we ever want an emoji, there is one: \U0001f4bb
	tab := strings.Repeat(" ", len(buildTitle))

	n, _ := strconv.ParseInt(h.BuildTime, 10, 64)
	buildTimeStr := time.Unix(n, 0).Format(time.UnixDate)

	buildTmpl := fmt.Sprintf("\n%s\n", buildTitle) +
		fmt.Sprintf("%s revision      %s\n", tab, h.BuildRevision) +
		fmt.Sprintf("%s datetime      %s\n", tab, buildTimeStr)

	if h.ShowGoRuntimeVersion {
		buildTmpl += fmt.Sprintf("%s runtime       %s\n", tab, runtime.Version())
	}

	return `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}
	{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}` + buildTmpl
}
