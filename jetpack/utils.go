package jetpack

import "bufio"
import "bytes"
import "compress/bzip2"
import "compress/gzip"
import "crypto/sha512"
import "encoding/json"
import "fmt"
import "io"
import "net"
import "os"
import "os/exec"
import "sort"

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

// Pretty-printing by resorting to JSON roundabout

type PPPrepper interface {
	PPPrepare() interface{}
}

func ppInner(obj interface{}, prefix string) {
	switch obj.(type) {
	case map[string]interface{}:
		m := obj.(map[string]interface{})

		if prefix != "" {
			prefix += "."
		}

		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			ppInner(m[k], prefix+k)
		}
	case []interface{}:
		for i, v := range obj.([]interface{}) {
			ppInner(v, fmt.Sprintf("%v[%d]", prefix, i))
		}
	default:
		fmt.Printf("%v = %#v\n", prefix, obj)
	}
}

func ReJSON(obj interface{}) (rv interface{}) {
	bb, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(bb, &rv)
	if err != nil {
		panic(err)
	}
	return rv
}

func PP(obj interface{}) {
	switch obj.(type) {
	case PPPrepper:
		obj = obj.(PPPrepper).PPPrepare()
	}
	ppInner(ReJSON(obj), "")
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
