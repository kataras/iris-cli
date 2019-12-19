package main

import (
	"github.com/kataras/iris-cli/project"
	"github.com/kataras/iris-cli/utils"
)

func main() {
	defer utils.ShowIndicator(nil)()

	p := project.New("./", "github.com/kataras/sitemap")
	p.Module = "github.com/author/my_project"
	// These should work too.
	// p.Dest = ""
	// AND
	// p.Dest = "./"
	// p.Module = ""
	// AND
	// p.Dest = ""
	// p.Module = ""

	if err := p.Install(); err != nil {
		print(err.Error())
	}
}
