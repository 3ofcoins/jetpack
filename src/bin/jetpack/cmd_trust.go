package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/appc/spec/schema/types"
	"github.com/juju/errors"

	"lib/fetch"
	"lib/keystore"
)

func runTrust(args []string) error {
	var root, doList, doDelete, yes bool
	var prefix types.ACName

	fl := flag.NewFlagSet("trust", flag.ExitOnError)
	fl.Var(&prefix, "prefix", "Force image name prefix")
	fl.BoolVar(&root, "root", false, "Root key (trust for all images)")
	fl.BoolVar(&doDelete, "d", false, "Delete key")
	fl.BoolVar(&doList, "l", false, "List trusted keys")
	fl.BoolVar(&yes, "yes", false, "Accept key without asking")
	fetch.AllowHTTPFlag(fl)

	fl.Parse(args)
	if root {
		if !prefix.Empty() {
			return errors.New("-root and -prefix can't be used together")
		}
		prefix = keystore.Root
	}
	args = fl.Args()

	ks := Host.Keystore()

	if doList || (len(args) == 0 && prefix == "" && !doDelete && !root) {
		kr, err := ks.GetAllKeys()
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
	} else if len(args) == 0 {
		return errors.New("Usage: ...")
	} else {
		// add key(s)
		for _, loc := range args {
			if name, kf, err := fetch.OpenPubKey(loc); err != nil {
				return errors.Trace(err)
			} else {
				defer kf.Close()

				usePrefix := prefix
				if prefix.Empty() {
					if name.Empty() {
						return errors.New("Unknown prefix, use -prefix or -root option")
					}
					usePrefix = name
				}

				if path, err := ks.StoreTrustedKey(usePrefix, kf, yes); err != nil {
					return errors.Trace(err)
				} else if path == "" {
					fmt.Println("Key NOT accepted")
				} else {
					fmt.Printf("Key accepted and saved at %v\n", path)
				}
			}
		}
	}

	return nil
}
