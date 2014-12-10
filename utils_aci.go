package jetpack

import "archive/tar"
import "compress/bzip2"
import "compress/gzip"
import "crypto/sha256"
import "encoding/base64"
import "errors"
import "fmt"
import "io"
import "io/ioutil"
import "os"
import "strings"

import "github.com/3ofcoins/rocket/app-container/aci"
import "github.com/3ofcoins/rocket/app-container/schema"

// FIXME: copy/paste from github.com/coreos/rocket/app-container/acutil/validate.go
func DecompressingReader(rs io.ReadSeeker) (io.Reader, error) {
	// TODO(jonboulle): this is a bit redundant with detectValType
	typ, err := aci.DetectFileType(rs)
	if err != nil {
		return nil, err
	}
	if _, err := rs.Seek(0, 0); err != nil {
		return nil, err
	}
	var r io.Reader
	switch typ {
	case aci.TypeGzip:
		r, err = gzip.NewReader(rs)
		if err != nil {
			return nil, fmt.Errorf("error reading gzip: %v", err)
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
		// should never happen
		panic(fmt.Sprintf("bad type returned from DetectFileType: %v", typ))
	}
	return r, nil
}

type ACI struct {
	Sha256 []byte
	schema.FilesetManifest
}

func (aci *ACI) Checksum() string {
	return fmt.Sprintf("sha256-%x", aci.Sha256)
}

func (aci *ACI) FSHash() string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(aci.Sha256), "=")
}

func ReadACI(path string) (*ACI, error) {
	aci := ACI{}

	zf, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer zf.Close()
	f, err := DecompressingReader(zf)
	if err != nil {
		return nil, err
	}
	hash := sha256.New()

	r := io.TeeReader(f, hash)
	tr := tar.NewReader(r)

	var fsmJSON []byte

TarLoop:
	for {
		switch hdr, err := tr.Next(); err {
		case nil:
			if hdr.Name == "fileset" {
				fsmJSON, err = ioutil.ReadAll(tr)
				if err != nil {
					return nil, err
				}
				break TarLoop
			}
		case io.EOF:
			break TarLoop
		default:
			return nil, err
		}
	}

	// Finish reading file, Tar may have read through last entry
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return nil, err
	}

	aci.Sha256 = hash.Sum(nil)

	if fsmJSON != nil {
		err = aci.FilesetManifest.UnmarshalJSON(fsmJSON)
		if err != nil {
			return nil, err
		}
	}

	return &aci, nil
}
