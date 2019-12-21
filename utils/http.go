package utils

import (
	"compress/gzip"
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if code := resp.StatusCode; code < 200 || code >= 400 {
		return nil, fmt.Errorf("resource not available <%s>: %s", url, resp.Status)
	}

	var reader io.Reader = resp.Body

	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	return ioutil.ReadAll(reader)
}
