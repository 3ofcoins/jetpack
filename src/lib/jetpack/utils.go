package jetpack

import "bufio"
import "bytes"
import "compress/bzip2"
import "compress/gzip"

import "fmt"
import "io"
import "net"

import "github.com/appc/spec/aci"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

const ACNoName = types.ACName("")

func untilError(steps ...func() error) error {
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// FIXME: mostly copy/paste from github.com/appc/spec/actool/validate.go
func DecompressingReader(rd io.Reader) (io.Reader, error) {
	brd := bufio.NewReaderSize(rd, 1024)
	header, err := brd.Peek(768)
	if err != nil {
		return nil, errors.Trace(err)
	}

	typ, err := aci.DetectFileType(bytes.NewReader(header))
	if err != nil {
		return nil, errors.Trace(err)
	}

	var r io.Reader
	switch typ {
	case aci.TypeGzip:
		r, err = gzip.NewReader(brd)
		if err != nil {
			return nil, errors.Trace(err)
		}
	case aci.TypeBzip2:
		r = bzip2.NewReader(brd)
	case aci.TypeXz:
		r = aci.XzReader(brd)
	case aci.TypeTar:
		r = brd
	case aci.TypeUnknown:
		return nil, errors.New("unknown filetype")
	default:
		panic(fmt.Sprintf("bad type returned from DetectFileType: %v", typ))
	}
	return r, nil
}

func ConsoleApp(username string) *types.App {
	return &types.App{
		Exec: []string{"/usr/bin/login", "-fp", username},
		User: "root",
	}
}

func nextIP(ip net.IP) net.IP {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i] += 1
		if ip[i] > 0 {
			return ip
		}
	}
	return nil
}
