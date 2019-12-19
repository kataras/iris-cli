package utils

import (
	"context"
	"io"
	"os"
	"time"
)

// ShowIndicator writes a loader to "w".
// Usage: defer utils.showIndicator(os.Stdout)()
func ShowIndicator(w io.Writer) func() {
	if w == nil {
		w = os.Stdout
	}

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
