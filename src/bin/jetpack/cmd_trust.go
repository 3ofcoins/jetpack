package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"golang.org/x/crypto/openpgp"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/keystore"
)

var allowHTTP bool

func runTrust(args []string) error {
	var prefix types.ACName
	var root, doList, doDelete bool

	fl := flag.NewFlagSet("trust", flag.ExitOnError)
	fl.Var(&prefix, "prefix", "Image name prefix")
	fl.BoolVar(&root, "root", false, "Root key (trust for all images)")
	fl.BoolVar(&doDelete, "d", false, "Delete key")
	fl.BoolVar(&doList, "l", false, "List trusted keys")
	fl.BoolVar(&allowHTTP, "insecure-allow-http", false, "allow HTTP use for key discovery and/or retrieval")

	die(fl.Parse(args))
	args = fl.Args()

	ks := Host.Keystore()

	if doList || (len(args) == 0 && prefix == "" && !doDelete && !root) {
		kr, err := ks.GetKeyring(keystore.Root)
		if err != nil {
			return errors.Trace(err)
		}

		if kr.Len() == 0 {
			fmt.Println("No trusted keys.")
			return nil
		}

		el := kr.Entities()
		sort.Sort(el)

		lines := make([]string, len(el))
		for i, ety := range el {
			lines[i] = ety.String()
		}

		w := tabwriter.NewWriter(os.Stdout, 2, 8, 2, ' ', 0)
		fmt.Fprintf(w, "PREFIX\tFINGERPRINT\tIDENTITY\n%v\n", strings.Join(lines, "\n"))
		die(w.Flush())
	} else if doDelete {
		if prefix != "" {
			return errors.New("untrust prefix not implemented yet")
		}
		if len(args) == 0 {
			return errors.New("Usage: ...")
		}
		for _, fprint := range args {
			fmt.Println("Untrusting:", fprint)
			if removed, err := ks.UntrustKey(fprint); err != nil {
				return errors.Trace(err)
			} else {
				fmt.Println("Removed from:", removed)
			}
		}
	} else {
		// Add key
		if len(args) == 0 {
			if prefix == "" {
				return errors.New("Usage: ...")
			}

			app, err := discovery.NewAppFromString(prefix.String())
			if err != nil {
				return errors.Trace(err)
			}

			ep, _, err := discovery.DiscoverPublicKeys(*app, allowHTTP)
			if err != nil {
				return errors.Trace(err)
			}

			args = ep.Keys
		}

		for _, location := range args {
			if f, err := openLocation(location); err != nil {
				return errors.Trace(err)
			} else {
				defer f.Close()
				if root {
					prefix = keystore.Root
				}
				if accepted, err := reviewKey(prefix, location, f, false); err != nil {
					return errors.Trace(err)
				} else if !accepted {
					fmt.Println("Key NOT accepted.")
				} else {
					if path, err := ks.StoreTrustedKey(prefix, f); err != nil {
						return errors.Trace(err)
					} else {
						fmt.Printf("Key accepted and saved at %v\n", path)
					}
				}
			}
		}
	}

	return nil
}

// TODO: downloader lib?
func openLocation(location string) (_ *os.File, erv error) {
	u, err := url.Parse(location)
	if err != nil {
		return nil, errors.Trace(err)
	}

	switch u.Scheme {
	case "":
		return os.Open("location")

	case "file":
		return os.Open(u.Path)

	case "http":
		if !allowHTTP {
			return nil, errors.New("-insecure-allow-http required for http URLs")
		}
		fallthrough

	case "https":
		// rkt/rkt/trust.go:downloadKey()
		tf, err := ioutil.TempFile("", "")
		if err != nil {
			return nil, errors.Errorf("error creating tempfile: %v", err)
		}
		os.Remove(tf.Name()) // no need to keep the tempfile around

		defer func() {
			if erv != nil {
				tf.Close()
			}
		}()

		res, err := http.Get(u.String())
		if err != nil {
			return nil, errors.Errorf("error getting key: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, errors.Errorf("bad HTTP status code: %d", res.StatusCode)
		}

		if _, err := io.Copy(tf, res.Body); err != nil {
			return nil, errors.Errorf("error copying key: %v", err)
		}

		tf.Seek(0, os.SEEK_SET)

		return tf, nil

	default:
		return nil, errors.Errorf("Unsupported scheme: %v\n", u.Scheme)
	}
}

// rkt/rkt/trust.go
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

func reviewKey(prefix types.ACName, location string, key *os.File, forceAccept bool) (bool, error) {
	defer key.Seek(0, os.SEEK_SET)

	kr, err := openpgp.ReadArmoredKeyRing(key)
	if err != nil {
		return false, errors.Trace(err)
	}

	fmt.Printf("Prefix: %q\nKey: %q\n", prefix, location)
	for _, k := range kr {
		fmt.Printf("GPG key fingerprint is: %s\n", fingerToString(k.PrimaryKey.Fingerprint))
		for _, sk := range k.Subkeys {
			fmt.Printf("    Subkey fingerprint: %s\n", fingerToString(sk.PublicKey.Fingerprint))
		}
		for n, _ := range k.Identities {
			fmt.Printf("\t%s\n", n)
		}
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
