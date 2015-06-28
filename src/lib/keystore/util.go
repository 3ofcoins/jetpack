package keystore

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/openpgp"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"
)

func fingerToString(fpr [20]byte) string {
	str := ""
	for i, b := range fpr {
		if i > 0 && i%2 == 0 {
			str += " "
			if i == 10 {
				str += " "
			}
		}
		str += strings.ToUpper(fmt.Sprintf("%.2x", b))
	}
	return str
}

func KeyDescription(ety *openpgp.Entity) string {
	rv := make([]string, 2+len(ety.Subkeys)+len(ety.Identities))
	rv[0] = fmt.Sprintf("GPG key fingerprint: %s", fingerToString(ety.PrimaryKey.Fingerprint))
	for i, sk := range ety.Subkeys {
		rv[i+1] = fmt.Sprintf(" Subkey fingerprint: %s", fingerToString(sk.PublicKey.Fingerprint))
	}
	i := len(ety.Subkeys) + 1
	rv[i] = "Identities:"
	for id := range ety.Identities {
		rv[i] = fmt.Sprintf(" - %v", id)
		i += 1
	}
	return strings.Join(rv, "\n")
}

func reviewKey(prefix types.ACIdentifier, key *os.File, forceAccept bool) (bool, error) {
	defer key.Seek(0, os.SEEK_SET)

	kr, err := openpgp.ReadArmoredKeyRing(key)
	if err != nil {
		return false, errors.Trace(err)
	}

	if prefix == Root {
		fmt.Println("Prefix: ROOT KEY (matches all names)")
	} else {
		fmt.Println("Prefix:", prefix)
	}
	for _, k := range kr {
		fmt.Println(KeyDescription(k))
	}

	if !forceAccept {
		in := bufio.NewReader(os.Stdin)
		for {
			fmt.Printf("Are you sure you want to trust this key (yes/no)? ")
			input, err := in.ReadString('\n')
			if err != nil {
				return false, errors.Errorf("error reading input: %v", err)
			}
			switch input {
			case "yes\n":
				return true, nil
			case "no\n":
				return false, nil
			default:
				fmt.Printf("Please enter 'yes' or 'no'")
			}
		}
	} else {
		fmt.Println("Warning: trust fingerprint verification has been disabled")
	}
	return true, nil
}

func pathToACIdentifier(path string) (types.ACIdentifier, error) {
	if dirname := filepath.Base(filepath.Dir(path)); dirname == "@" {
		return Root, nil
	} else if prefix, err := types.NewACIdentifier(strings.Replace(dirname, ",", "/", -1)); err != nil {
		return "", err
	} else {
		return *prefix, nil
	}
}

func fingerprintToFilename(fp [20]byte) string {
	return fmt.Sprintf("%x", fp)
}
