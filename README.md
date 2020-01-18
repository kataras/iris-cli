# Iris CLI (Work In Progress)

[![build status](https://img.shields.io/travis/kataras/iris-cli/master.svg?style=for-the-badge&logo=travis)](https://travis-ci.org/kataras/iris-cli) [![report card](https://img.shields.io/badge/report%20card-a%2B-ff3333.svg?style=for-the-badge)](https://goreportcard.com/report/github.com/kataras/iris-cli)

Iris Command Line Interface is your buddy when it comes to get started with [Iris](https://github.com/kataras/iris) and [Go](https://golang.org/dl).

![](https://iris-go.com/images/iris-cli-screen.png)

> This project is not finished. It is under active development. **TEST ONLY**

## Installation

The only requirement is the [Go Programming Language](https://golang.org/dl).

```sh
go get github.com/kataras/iris-cli
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

### New Command

```sh
iris-cli new [--module=my_app] react-typescript
#                              svelte
```

### Run Command

```sh
iris-cli run
# optional argument, the project directory or
# a project template.
```

[Download, install](#new-command) and run a [project template](registry.yml) at once.

```sh
iris-cli run react-typescript
```

### Clean Command

```sh
iris-cli clean
# optional argument, the project directory,
# defaults to the current working directory.
```

### Unistall Command

```sh
iris-cli unistall
# optional argument, the project directory,
# defaults to the current working directory.
```

### Init Command

Create a new local iris project file through a local git repository.

```sh
iris-cli init
```

### Add Command

```sh
iris-cli add file.go
```

```sh
iris-cli add [--repo=iris-contrib/snippets] [--pkg=my_package] [--data=repo.json] [--replace=oldValue=newValue,oldValue2=newValue2] file.go[@version]
```

### Check Command

```sh
iris-cli check [module]  
#              [iris]
#              [gopkg.in/yaml.v2]
#              [all]
```

### Contributing

We'd love to see your contribution to the Iris CLI! For more information about contributing to the Iris Command Line Interface please check the [CONTRIBUTING.md](CONTRIBUTING.md) file.

[List of all Contributors](https://github.com/kataras/iris-cli/graphs/contributors)

## License

Iris CLI is free and open-source software licensed under the [MIT License](LICENSE).
