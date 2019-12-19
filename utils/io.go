package utils

import (
	"io"
)

// NoOpReadCloser wraps the "r" and returns a new io.ReadCloser which its `Close` does nothing.
func NoOpReadCloser(r io.Reader) io.ReadCloser {
	return noOpCloser{r}
}

type noOpCloser struct {
	io.Reader
}

func (r noOpCloser) Close() error { return nil }
