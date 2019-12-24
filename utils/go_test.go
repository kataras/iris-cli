package utils

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestPackage(t *testing.T) {
	contents := []byte(`package main
func main() {}`)

	if expected, got := []byte("main"), Package(contents); !bytes.Equal(expected, got) {
		t.Fatalf("expected %q but got %q", expected, got)
	}
}

func TestModulePath(t *testing.T) {
	contents := []byte(`module testmodule
require github.com/kataras/iris v12.1.2
`)

	if expected, got := []byte("testmodule"), ModulePath(contents); !bytes.Equal(expected, got) {
		t.Fatalf("expected %q but got %q", expected, got)
	}
}

func TestTryFindPackage(t *testing.T) {
	contents := []byte(`package main
func main() {}`)

	f, err := ioutil.TempFile("", "*.go")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = f.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Dir(f.Name())

	if expected, got := []byte("main"), TryFindPackage(dir); !bytes.Equal(expected, got) {
		t.Fatalf("expected %q but got %q", expected, got)
	}
}
