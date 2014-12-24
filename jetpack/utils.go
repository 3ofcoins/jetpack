package jetpack

import "bufio"
import "bytes"
import "compress/bzip2"
import "compress/gzip"
import "crypto/sha512"
import "fmt"
import "io"
import "net"
import "os"
import "os/exec"

import "github.com/appc/spec/aci"
import "github.com/appc/spec/schema/types"
import "github.com/juju/errors"

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

func UnpackImage(uri, path string) (types.Hash, error) {
	// use fetch(1) to avoid worrying about protocols, proxies and such
	fetchCmd := exec.Command("fetch", "-o", "-", uri)
	fetchCmd.Stderr = os.Stderr
	fetch, err := fetchCmd.StdoutPipe()
	if err != nil {
		return types.Hash{}, errors.Trace(err)
	}

	// We trust system's tar, no need to roll our own
	untarCmd := exec.Command("tar", "-C", path, "-xf", "-")
	untarCmd.Stderr = os.Stderr
	untar, err := untarCmd.StdinPipe()
	if err != nil {
		return types.Hash{}, errors.Trace(err)
	}

	if err := untarCmd.Start(); err != nil {
		return types.Hash{}, errors.Trace(err)
	}
	// FIXME: defer killing process if survived

	if err := fetchCmd.Start(); err != nil {
		return types.Hash{}, errors.Trace(err)
	}
	// FIXME: defer killing process if survived

	aci, err := DecompressingReader(fetch)
	if err != nil {
		return types.Hash{}, errors.Trace(err)
	}

	hash := sha512.New()

	if _, err := io.Copy(hash, io.TeeReader(aci, untar)); err != nil {
		return types.Hash{}, errors.Trace(err)
	}

	if err := fetch.Close(); err != nil {
		return types.Hash{}, errors.Trace(err)
	}

	if err := fetchCmd.Wait(); err != nil {
		return types.Hash{}, errors.Trace(err)
	}

	if err := untar.Close(); err != nil {
		return types.Hash{}, errors.Trace(err)
	}

	if err := untarCmd.Wait(); err != nil {
		return types.Hash{}, errors.Trace(err)
	}

	if hash, err := types.NewHash(fmt.Sprintf("sha512-%x", hash.Sum(nil))); err != nil {
		// CAN'T HAPPEN
		return types.Hash{}, errors.Trace(err)
	} else {
		return *hash, nil
	}
}

func ConsoleApp(username string) *types.App {
	return &types.App{
		Exec: []string{"/usr/bin/login", "-f", username},
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
	panic("RAN OUT OF IPS")
}
