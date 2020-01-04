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

    /* $ command 4
    $ command5
        $ command6
    */

    app.HandleDir("/", "./app/build_literal", iris.DirOptions{
        Asset:      Asset,
        AssetNames: AssetNames,
        AssetInfo:  AssetInfo,
    })

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
		"command 4",
		"command5",
		"command6",
	}
	if !reflect.DeepEqual(res.Commands, expectedCommands) {
		t.Fatalf("expected parsed commands to be:\n<%s>\nbut got:\n<%s>",
			strings.Join(res.Commands, ", "), strings.Join(expectedCommands, ", "))
	}

	expectedAssetDirs := []AssetDir{
		{Dir: "./app/build_literal", ShouldGenerated: true},
		{Dir: "./app/build_var", ShouldGenerated: true},
		{Dir: "./public", ShouldGenerated: false},
	}
	if !reflect.DeepEqual(res.AssetDirs, expectedAssetDirs) {
		t.Fatalf("expected parsed asset targets to be:\n<%v>\nbut got:\n<%v>",
			res.AssetDirs, expectedAssetDirs)
	}
}
