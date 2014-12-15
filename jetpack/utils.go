package jetpack

import "archive/tar"
import "compress/bzip2"
import "compress/gzip"
import "crypto/sha256"
import "encoding/base64"
import "fmt"
import "io"
import "io/ioutil"
import "os"
import "os/exec"
import "reflect"
import "strings"

import "github.com/appc/spec/aci"
import "github.com/appc/spec/schema"
import "github.com/appc/spec/schema/types"
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

type ACI struct {
	Sha256 []byte
	Path   string
	schema.ImageManifest
}

func ReadACI(path string) (*ACI, error) {
	aci := &ACI{Path: path}

	zf, err := os.Open(path)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer zf.Close()
	f, err := DecompressingReader(zf)
	if err != nil {
		return nil, errors.Trace(err)
	}
	hash := sha256.New()

	r := io.TeeReader(f, hash)
	tr := tar.NewReader(r)

	var manifestJSON []byte

TarLoop:
	for {
		switch hdr, err := tr.Next(); err {
		case nil:
			if hdr.Name == "manifest" {
				manifestJSON, err = ioutil.ReadAll(tr)
				if err != nil {
					return nil, errors.Trace(err)
				}
				break TarLoop
			}
		case io.EOF:
			break TarLoop
		default:
			return nil, errors.Trace(err)
		}
	}

	// Finish reading file, Tar may have read through last entry
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return nil, errors.Trace(err)
	}

	aci.Sha256 = hash.Sum(nil)

	if manifestJSON != nil {
		err = aci.ImageManifest.UnmarshalJSON(manifestJSON)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	return aci, nil
}

func (aci *ACI) Checksum() string {
	return fmt.Sprintf("sha256-%x", aci.Sha256)
}

func (aci *ACI) Checksum64() string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(aci.Sha256), "=")
}

func (aci *ACI) String() string {
	version, _ := aci.Get("version")
	os, _ := aci.Get("os")
	arch, _ := aci.Get("arch")
	return fmt.Sprintf("%v-%v-%v-%v.aci[%v]",
		aci.Name, version, os, arch,
		aci.Checksum())
}

func (aci *ACI) IsApp() bool {
	return !reflect.DeepEqual(aci.App, types.App{})
}
