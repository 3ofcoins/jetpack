package fetch

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/coreos/ioprogress"
	"github.com/juju/errors"
)

func ProgressBarReader(r io.Reader, size int64) io.Reader {
	if size > 0 && size < 5120 { // TODO: isatty
		// Don't bother with progress bar below 50k
		return r
	}

	bar := ioprogress.DrawTextFormatBar(int64(56))
	fmtfunc := func(progress, total int64) string {
		// Content-Length is set to -1 when unknown.
		if total == -1 {
			return fmt.Sprintf(
				"Progress: %v/?",
				ioprogress.ByteUnitStr(progress),
			)
		}
		return fmt.Sprintf(
			"Progress: %s %s",
			bar(progress, total),
			ioprogress.DrawTextFormatBytes(progress, total),
		)
	}
	return &ioprogress.Reader{
		Reader:       r,
		Size:         size,
		DrawFunc:     ioprogress.DrawTerminalf(os.Stdout, fmtfunc),
		DrawInterval: time.Second,
	}
}

func ProgressBarFileReader(f *os.File) io.Reader {
	if fi, err := f.Stat(); err != nil {
		panic(err)
	} else {
		return ProgressBarReader(f, fi.Size())
	}
}

func OpenURL(url string) (_ *os.File, erv error) {
	tf, err := ioutil.TempFile("", "jetpack.fetch.")
	if err != nil {
		return nil, errors.Trace(err)
	}
	os.Remove(tf.Name()) // no need to keep the tempfile around

	defer func() {
		if erv != nil {
			tf.Close()
		}
	}()

	res, err := http.Get(url)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.Errorf("bad HTTP status code: %d", res.StatusCode)
	}

	fmt.Println("Downloading", url, "...")
	if _, err := io.Copy(tf, ProgressBarReader(res.Body, res.ContentLength)); err != nil {
		return nil, errors.Trace(err)
	}

	tf.Seek(0, os.SEEK_SET)

	return tf, nil
}

const flagAllowHTTP = false // TODO: make the flag

func OpenLocation(location string) (_ *os.File, erv error) {
	if location == "-" {
		return os.Stdin, nil
	}

	u, err := url.Parse(location)
	if err != nil {
		return nil, errors.Trace(err)
	}

	switch u.Scheme {
	case "":
		return os.Open(location)

	case "file":
		return os.Open(u.Path)

	case "http":
		if !flagAllowHTTP {
			return nil, errors.New("-insecure-allow-http required for http URLs")
		}
		fallthrough

	case "https":
		return OpenURL(u.String())

	default:
		return nil, errors.Errorf("Unsupported scheme: %v\n", u.Scheme)
	}
}
