module github.com/kataras/iris-cli

go 1.13

require (
	github.com/AlecAivazis/survey/v2 v2.0.5
	github.com/BurntSushi/toml v0.3.1
	github.com/cheggaaa/pb/v3 v3.0.3
	github.com/fsnotify/fsnotify v1.4.8-0.20191012010759-4bf2d1fec783
	github.com/spf13/cobra v0.0.5
	golang.org/x/sys v0.0.0-20191228213918-04cbcbbfeed8
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/AlecAivazis/survey/v2 => github.com/kataras/survey/v2 v2.0.6
