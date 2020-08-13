# Iris CLI (Work In Progress)

[![build status](https://img.shields.io/travis/kataras/iris-cli/master.svg?style=for-the-badge&logo=travis)](https://travis-ci.org/kataras/iris-cli) [![report card](https://img.shields.io/badge/report%20card-a%2B-ff3333.svg?style=for-the-badge)](https://goreportcard.com/report/github.com/kataras/iris-cli)

Iris Command Line Interface is your buddy when it comes to get started with [Iris](https://github.com/kataras/iris) and [Go](https://golang.org/dl).

![](https://iris-go.com/images/iris-cli-screen.png)

> This project is not finished. It is under active development. **TEST ONLY**

## Installation

The only requirement is the [Go Programming Language](https://golang.org/dl).

```sh
$ go get github.com/kataras/iris-cli
```

## Troubleshooting

If you get a network error during installation please make sure you set a valid [GOPROXY](https://github.com/golang/go/wiki/Modules#are-there-always-on-module-repositories-and-enterprise-proxies) environment variable.

```sh
$ go env -w GOPROXY=https://goproxy.cn,https://gocenter.io,https://goproxy.io,direct
```

If you get a network error during `iris-cli` execution, retry with the `--proxy` global flag.

```sh
$ iris-cli --proxy=env [COMMAND] [FLAGS]
#          --proxy=119.28.233.135:8080
```

[List all Releases](https://github.com/kataras/iris-cli/releases)

## Table of Contents

* Project Commands
    * [new](#new-command)
    * [run](#run-command)
    * [clean](#clean-command)
    * [unistall](#unistall-command)
    * [init](#init-command)
* Snippet Commands
    * [add](#add-command)
* Miscellaneous
    * [check](#check-command)
    * [stats](#stats-command)

### New Command

```sh
$ iris-cli new [--module=my_app] react-typescript
#                              svelte
```

### Run Command

```sh
$ iris-cli run
# optional argument, the project directory or
# a project template.
```

[Download, install](#new-command) and run a [project template](registry.yml) at once.

```sh
$ iris-cli run react-typescript
```

### Clean Command

```sh
$ iris-cli clean
# optional argument, the project directory,
# defaults to the current working directory.
```

### Unistall Command

```sh
$ iris-cli unistall
# optional argument, the project directory,
# defaults to the current working directory.
```

### Init Command

Create a new local iris project file through a local git repository.

```sh
$ iris-cli init
```

### Add Command

```sh
$ iris-cli add file.go
```

```sh
$ iris-cli add [--repo=iris-contrib/snippets] [--pkg=my_package] [--data=repo.json] [--replace=oldValue=newValue,oldValue2=newValue2] file.go[@version]
```

### Check Command

```sh
$ iris-cli check [module]  
#              [iris]
#              [gopkg.in/yaml.v2]
#              [all]
```

### Stats Command

Stats command shows stats for a collection of modules based on the
major Go Proxies (goproxy.cn, gocenter.io, goproxy.io). Modules are separated by spaces.

#### Get Download Count

Download count per GOPROXY for a module and total for repository.

```sh
$ iris-cli stats --download-count [modules]
#              github.com/kataras/iris github.com/kataras/iris/v12
#              gopkg.in/yaml.v3 gopkg.in/yaml.v2

[github.com/kataras/iris]
• goproxy.cn: 27474
• gocenter.io: 5560
• total: 33034
[github.com/kataras/iris/v12]
• goproxy.cn: 33589
• gocenter.io: 3024
• total: 36613
[gopkg.in/yaml.v2]
• goproxy.cn: 2306257
• gocenter.io: 1686035
• total: 3992292
[gopkg.in/yaml.v3]
• goproxy.cn: 241121
• gocenter.io: 37909
• total: 279030

[repository total]
• github.com/kataras/iris: 69647
• gopkg.in/yaml: 4271322
```

### List Versions

List all available releases Go Proxies have cached.

```sh
$ iris-cli stats --versions github.com/aws/copilot-cli gopkg.in/yaml.v2

[github.com/aws/copilot-cli]
• goproxy.io:
  • v0.0.4
  • v0.0.5
  • v0.0.6
  • v0.0.7
  • v0.0.8
  • v0.0.9
  • v0.1.0
  • v0.2.0
[gopkg.in/yaml.v2]
• goproxy.io:
  • v2.0.0
  • v2.1.0
  • v2.1.1
  • v2.2.0
  • v2.2.1
  • v2.2.2
  • v2.2.3
  • v2.2.4
  • v2.2.5
  • v2.2.6
  • v2.2.7
  • v2.2.8
  • v2.3.0
```

### Contributing

We'd love to see your contribution to the Iris CLI! For more information about contributing to the Iris Command Line Interface please check the [CONTRIBUTING.md](CONTRIBUTING.md) file.

[List of all Contributors](https://github.com/kataras/iris-cli/graphs/contributors)

## License

Iris CLI is free and open-source software licensed under the [MIT License](LICENSE).
