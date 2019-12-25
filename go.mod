module github.com/kataras/iris-cli

go 1.13

require (
	github.com/AlecAivazis/survey/v2 v2.0.5
	github.com/BurntSushi/toml v0.3.1
	github.com/cheggaaa/pb/v3 v3.0.3
	github.com/spf13/cobra v0.0.5
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/AlecAivazis/survey/v2 => github.com/kataras/survey/v2 v2.0.6
