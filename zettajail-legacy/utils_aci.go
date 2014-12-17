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
import "runtime"
import "strings"

import "github.com/3ofcoins/appc-spec/aci"
import "github.com/3ofcoins/appc-spec/schema"
import "github.com/3ofcoins/appc-spec/schema/types"

// FIXME: copy/paste from github.com/coreos/rocket/app-container/acutil/validate.go
func DecompressingReader(rs io.ReadSeeker) (io.Reader, error) {
	//DONE 	// TODO(jonboulle): this is a bit redundant with detectValType
	//DONE 	typ, err := aci.DetectFileType(rs)
	//DONE 	if err != nil {
	//DONE 		return nil, err
	//DONE 	}
	//DONE 	if _, err := rs.Seek(0, 0); err != nil {
	//DONE 		return nil, err
	//DONE 	}
	//DONE 	var r io.Reader
	//DONE 	switch typ {
	//DONE 	case aci.TypeGzip:
	//DONE 		r, err = gzip.NewReader(rs)
	//DONE 		if err != nil {
	//DONE 			return nil, fmt.Errorf("error reading gzip: %v", err)
	//DONE 		}
	//DONE 	case aci.TypeBzip2:
	//DONE 		r = bzip2.NewReader(rs)
	//DONE 	case aci.TypeXz:
	//DONE 		r = aci.XzReader(rs)
	//DONE 	case aci.TypeTar:
	//DONE 		r = rs
	//DONE 	case aci.TypeUnknown:
	//DONE 		return nil, errors.New("unknown filetype")
	//DONE 	default:
	//DONE 		// should never happen
	//DONE 		panic(fmt.Sprintf("bad type returned from DetectFileType: %v", typ))
	//DONE 	}
	//DONE 	return r, nil
	//DONE
	return nil, nil
}

type ACI struct {
	Sha256 []byte
	schema.ImageManifest
}

func (aci *ACI) Checksum() string {
	//DONE return fmt.Sprintf("sha256-%x", aci.Sha256)
	return ""
}

func (aci *ACI) FSHash() string {
	//DONE return strings.TrimRight(base64.URLEncoding.EncodeToString(aci.Sha256), "=")
	return ""
}

func ReadACI(path string) (*ACI, error) {
	//DONE 	aci := ACI{}
	//DONE
	//DONE 	zf, err := os.Open(path)
	//DONE 	if err != nil {
	//DONE 		return nil, err
	//DONE 	}
	//DONE 	defer zf.Close()
	//DONE 	f, err := DecompressingReader(zf)
	//DONE 	if err != nil {
	//DONE 		return nil, err
	//DONE 	}
	//DONE 	hash := sha256.New()
	//DONE
	//DONE 	r := io.TeeReader(f, hash)
	//DONE 	tr := tar.NewReader(r)
	//DONE
	//DONE 	var fsmJSON []byte
	//DONE
	//DONE TarLoop:
	//DONE 	for {
	//DONE 		switch hdr, err := tr.Next(); err {
	//DONE 		case nil:
	//DONE 			if hdr.Name == "manifest" {
	//DONE 				fsmJSON, err = ioutil.ReadAll(tr)
	//DONE 				if err != nil {
	//DONE 					return nil, err
	//DONE 				}
	//DONE 				break TarLoop
	//DONE 			}
	//DONE 		case io.EOF:
	//DONE 			break TarLoop
	//DONE 		default:
	//DONE 			return nil, err
	//DONE 		}
	//DONE 	}
	//DONE
	//DONE 	// Finish reading file, Tar may have read through last entry
	//DONE 	if _, err := io.Copy(ioutil.Discard, r); err != nil {
	//DONE 		return nil, err
	//DONE 	}
	//DONE
	//DONE 	aci.Sha256 = hash.Sum(nil)
	//DONE
	//DONE 	if fsmJSON != nil {
	//DONE 		err = aci.ImageManifest.UnmarshalJSON(fsmJSON)
	//DONE 		if err != nil {
	//DONE 			return nil, err
	//DONE 		}
	//DONE 	}
	//DONE
	//DONE 	return &aci, nil
	//DONE
	return nil, nil
}

func NewImageManifest(name string) *schema.ImageManifest {
	//DONE 	// Cannot validate, assertValid is not exported, so we return as is.
	//DONE 	return &schema.ImageManifest{
	//DONE 		ACKind:    types.ACKind("ImageManifest"),
	//DONE 		ACVersion: types.SemVer{Major: 0, Minor: 1, Patch: 0},
	//DONE 		Name:      types.ACName(name),
	//DONE 		Labels: types.Labels{
	//DONE 			types.Label{"os", runtime.GOOS},
	//DONE 			types.Label{"arch", runtime.GOARCH},
	//DONE 		},
	//DONE 	}
	return nil
}
