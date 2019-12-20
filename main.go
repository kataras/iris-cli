package main

import (
	"fmt"
	"os"

	// "github.com/kataras/iris-cli/project"
	// "github.com/kataras/iris-cli/utils"

	// "github.com/AlecAivazis/survey/v2"
	"github.com/kataras/iris-cli/cmd"
)

const (
	// buildRevision is the build revision (docker commit string or git rev-parse HEAD) but it's
	// available only on the build state, on the cli executable - via the "--version" flag.
	buildRevision = ""
	// buildTime is the build unix time (in seconds since 1970-01-01 00:00:00 UTC), like the `buildRevision`,
	// this is available on after the build state, inside the cli executable - via the "--version" flag.
	//
	// Note that this buildTime is not int64, it's type of string and it is provided at build time.
	// Do not change!
	buildTime    = ""
	buildVersion = ""
)

func main() {
	// defer utils.ShowIndicator(nil)()

	// p := project.New("./", "github.com/kataras/sitemap")
	// p.Module = "github.com/author/my_project"
	// // These should work too.
	// // p.Dest = ""
	// // AND
	// // p.Dest = "./"
	// // p.Module = ""
	// // AND
	// // p.Dest = ""
	// // p.Module = ""

	// if err := p.Install(); err != nil {
	// 	print(err.Error())
	// }

	app := cmd.New(buildRevision, buildTime, buildVersion)
	if err := app.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
