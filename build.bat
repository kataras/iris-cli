set executable=iris-cli
set output=./bin
set input=./main.go

REM disable CGO
set CGO_ENABLED=0

ECHO Building windows binaries...
REM windows-x64
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o %output%/%executable%-windows-amd64.exe %input%
REM windows-x86
set GOOS=windows
set GOARCH=386
go build -ldflags="-s -w" -o %output%/%executable%-windows-386.exe %input%

ECHO Building linux binaries...
REM linux-x64
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o %output%/%executable%-linux-amd64 %input%
REM linux-x86
set GOOS=linux
set GOARCH=386
go build -ldflags="-s -w" -o %output%/%executable%-linux-386 %input%
REM linux-arm64
set GOOS=linux
set GOARCH=arm64
go build -ldflags="-s -w" -o %output%/%executable%-linux-arm64 %input%
REM linux-arm
set GOOS=linux
set GOARCH=arm
go build -ldflags="-s -w" -o %output%/%executable%-linux-arm %input%

ECHO Building darwin (osx) x64 binary...
REM darwin-x64
set GOOS=darwin
set GOARCH=amd64
go build -ldflags="-s -w" -o %output%/%executable%-darwin-amd64 %input%