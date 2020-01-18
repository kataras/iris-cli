package parser

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	src := `package main
    
const assetsDirectory = "./app/build_var"

func main(){
    app := iris.New()

    /* $ command1
    $ command2 */

    // $ command3

    /* $ command arg
    $ command5
        $ command6
    */

    app.HandleDir("/", "./app/build_literal", iris.DirOptions{
        Asset:      Asset,
        AssetNames: AssetNames,
        AssetInfo:  AssetInfo,
    })

	// $ command arg1 arg2
    app.HandleDir("/", assetsDirectory, iris.DirOptions{
        Asset:      Asset,
        AssetNames: AssetNames,
        AssetInfo:  AssetInfo,
    })

    app.HandleDir("/", "./public")
}
`
	res, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}

	expectedCommands := []string{
		"command1",
		"command2",
		"command3",
		"command arg",
		"command5",
		"command6",
		"command arg1 arg2",
	}

	for i, cmd := range res.Commands {
		nameArgs := strings.Split(expectedCommands[i], " ")

		if expected, got := nameArgs[0], cmd.Name; expected != got {
			t.Fatalf("[%d] expected parsed command to be: %s but got: %s", i, expected, got)
		}

		if expected, got := len(nameArgs[1:]), len(cmd.Args); expected != got {
			t.Fatalf("[%d] expected parsed command args length to be: %d but got: %d", i, expected, got)
		}

		if expected, got := strings.Join(nameArgs[1:], " "), strings.Join(cmd.Args, " "); !reflect.DeepEqual(expected, got) {
			t.Fatalf("[%d] expected parsed command args to be: %s but got: %s", i, expected, got)
		}

	}

	expectedAssetDirs := []*AssetDir{
		{Dir: "./app/build_literal", ShouldGenerated: true},
		{Dir: "./app/build_var", ShouldGenerated: true},
		{Dir: "./public", ShouldGenerated: false},
	}
	if !reflect.DeepEqual(res.AssetDirs, expectedAssetDirs) {
		t.Fatalf("expected parsed asset targets to be:\n<%v>\nbut got:\n<%v>",
			res.AssetDirs, expectedAssetDirs)
	}
}
