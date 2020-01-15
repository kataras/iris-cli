package utils

import (
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

// DownloadOption is the type of third, variadic input argument of the `Download` package-level function.
// Could change or read the request before use.
type DownloadOption func(*http.Request) error

// Download returns the body of "url".
// It uses the `http.DefaultClient` to download the resource specified by the "url" input argument.
func Download(url string, body io.Reader, options ...DownloadOption) ([]byte, error) {
	r, err := DownloadReader(url, body, options...)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ioutil.ReadAll(r)
}

var httpClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: IsInsideDocker(),
		},
	},
}

// DownloadReader returns a response reader.
func DownloadReader(url string, body io.Reader, options ...DownloadOption) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept-Encoding", "gzip")

	for _, opt := range options {
		if err = opt(req); err != nil {
			return nil, err
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	// defer resp.Body.Close()
	var reader io.ReadCloser = resp.Body

	if code := resp.StatusCode; code < 200 || code >= 400 {
		reader.Close()
		return nil, fmt.Errorf("resource not available <%s>: %s", url, resp.Status)
	}

	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(reader)
		if err != nil {
			reader.Close()
			return nil, err
		}

		// defer gzipReader.Close()
		reader = multiCloser{Reader: gzipReader, closers: []io.ReadCloser{gzipReader, reader}}
	}

	return reader, nil
}

// ListReleases lists all releases of a github "repo".
func ListReleases(repo string) []string {
	resp := []struct {
		TagName string `json:"tag_name"`
	}{}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases", repo)
	b, err := Download(url, nil)
	if err != nil {
		return nil
	}

	if err := json.Unmarshal(b, &resp); err != nil {
		return nil
	}

	releases := make([]string, 0, len(resp))
	for _, v := range resp {
		releases = append(releases, v.TagName)
	}

	return releases
}

// DownloadFile returns the contents of a github file inside a repository.
func DownloadFile(repo, version, name string) ([]byte, error) {
	if version == "" || version == "latest" {
		version = "master"
	}

	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repo, version, name)
	return Download(url, nil)
}
