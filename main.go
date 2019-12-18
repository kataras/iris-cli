package main

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	p := NewRemoteProject("github.com/kataras/iris")
	err := p.Download("./", "github.com/author/project")
	if err != nil {
		panic(err)
	}
}

type Application struct {
}

type RemoteProject struct {
	Repo       string // without https://.
	Branch     string // defautls to "master".
	ModuleName string // defaults to Repo's value.
}

func NewRemoteProject(repo string) *RemoteProject {
	return &RemoteProject{
		Repo:       repo,
		Branch:     "master",
		ModuleName: repo,
	}
}

func (p *RemoteProject) Download(dest, newModuleName string) error {
	defer showIndicator(os.Stdout)()

	gopath := os.Getenv("GOPATH")
	if dest == "" {
		if gopath != "" {
			dest = filepath.Join(gopath, "src", filepath.Dir(newModuleName))
		} else {
			dest, _ = os.Getwd()
		}
	} else {
		dest = strings.Replace(dest, "${GOPATH}", gopath, 1)
		d, err := filepath.Abs(dest)
		if err == nil {
			dest = d
		}
	}

	zipURL := fmt.Sprintf("https://%s/archive/%s.zip", p.Repo, p.Branch)
	req, err := http.NewRequest(http.MethodGet, zipURL, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Accept-Encoding", "gzip")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// println(resp.Header.Get("Content-Length"))
	// println(resp.ContentLength)

	var reader io.Reader = resp.Body

	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	//	respReader := &responseReader{Reader: reader, length: resp.ContentLength}

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}

	localProject := LocalProject{
		Path:               dest,
		OriginalModuleName: p.ModuleName,
		ModuleName:         newModuleName,
		OriginalRootDir:    filepath.Base(p.Repo) + "-" + p.Branch, // e.g. iris-master
	}

	return localProject.Unzip(bytes.NewReader(body), int64(len(body)))
}

type LocalProject struct {
	Source             string
	Path               string
	OriginalRootDir    string
	OriginalModuleName string
	ModuleName         string
}

func (p *LocalProject) Unzip(reader io.ReaderAt, size int64) error {
	r, err := zip.NewReader(reader, size)
	if err != nil {
		return err
	}

	oldModuleName := []byte(p.OriginalModuleName)
	newModuleName := []byte(p.ModuleName)
	shouldReplace := !bytes.Equal(oldModuleName, newModuleName)

	for _, f := range r.File {
		// Store filename/path for returning and using later on
		fpath := filepath.Join(p.Path, f.Name)

		// https://snyk.io/research/zip-slip-vulnerability#go
		if !strings.HasPrefix(fpath, filepath.Clean(p.Path)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		var rc io.ReadCloser

		rc, err = f.Open()
		if err != nil {
			return err
		}

		if shouldReplace {
			contents, err := ioutil.ReadAll(rc)
			if err != nil {
				return err
			}

			newContents := bytes.ReplaceAll(contents, oldModuleName, newModuleName)
			rc = noOpCloser{bytes.NewReader(newContents)}
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	newpath := filepath.Join(p.Path, filepath.Base(p.ModuleName))
	os.RemoveAll(newpath)
	return os.Rename(filepath.Join(p.Path, p.OriginalRootDir), newpath)
}

type noOpCloser struct {
	io.Reader
}

func (r noOpCloser) Close() error { return nil }

func showIndicator(w io.Writer) func() {
	ctx, cancel := context.WithCancel(context.TODO())

	go func() {
		w.Write([]byte("|"))
		w.Write([]byte("_"))
		w.Write([]byte("|"))
		for {
			select {
			case <-ctx.Done():
				return
			default:
				w.Write([]byte("\010\010-"))
				time.Sleep(time.Second / 2)
				w.Write([]byte("\010\\"))
				time.Sleep(time.Second / 2)
				w.Write([]byte("\010|"))
				time.Sleep(time.Second / 2)
				w.Write([]byte("\010/"))
				time.Sleep(time.Second / 2)
				w.Write([]byte("\010-"))
				time.Sleep(time.Second / 2)
				w.Write([]byte("|"))
			}
		}
	}()

	return func() {
		cancel()
		w.Write([]byte("\010\010\010")) //remove the loading chars
	}
}
