package jetpack

import "compress/bzip2"
import "compress/gzip"
import "fmt"
import "io"
import "os"
import "os/exec"

import "github.com/appc/spec/aci"
import "github.com/juju/errors"

func runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func bool2zfs(fl bool) string {
	if fl {
		return "on"
	} else {
		return "off"
	}
}

// FIXME: mostly copy/paste from github.com/appc/spec/actool/validate.go
func DecompressingReader(rs io.ReadSeeker) (io.Reader, error) {
	typ, err := aci.DetectFileType(rs)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if _, err := rs.Seek(0, 0); err != nil {
		return nil, errors.Trace(err)
	}
	var r io.Reader
	switch typ {
	case aci.TypeGzip:
		r, err = gzip.NewReader(rs)
		if err != nil {
			return nil, errors.Trace(err)
		}
	case aci.TypeBzip2:
		r = bzip2.NewReader(rs)
	case aci.TypeXz:
		r = aci.XzReader(rs)
	case aci.TypeTar:
		r = rs
	case aci.TypeUnknown:
		return nil, errors.New("unknown filetype")
	default:
		panic(fmt.Sprintf("bad type returned from DetectFileType: %v", typ))
	}
	return r, nil
}
