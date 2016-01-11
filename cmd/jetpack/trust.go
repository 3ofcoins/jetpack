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
)

func init() {
	AddCommand("trust [LOCATION]", "Trust or list ACI signing keys", cmdTrust, flTrust)
	AddCommand("untrust KEY...", "Remove keys from trust database", cmdUntrust, nil)
}

var trustPrefix types.ACIdentifier
var trustRoot bool
var trustFingerprint string

func flTrust(fl *flag.FlagSet) {
	fl.Var(&trustPrefix, "prefix", "Force image name prefix")
	fl.BoolVar(&trustRoot, "root", false, "Root key (matches all images)")
	fl.StringVar(&trustFingerprint, "fingerprint", "", "Specify key fingerprint to accept")
}

func cmdTrust(args []string) error {
	if len(args) == 0 {
		return errors.Trace(listKeys())
	}
	return errors.Trace(trustKeys(args))
}

func cmdUntrust(args []string) error {
	if len(args) == 0 {
		return ErrUsage
	}
	return untrustKeys(args)
}

func listKeys() error {
	ks := Host.Keystore()

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
	return errors.Trace(w.Flush())
}

func trustKeys(args []string) error {
	for _, loc := range args {
		if trustPrefix.Empty() {
			if acnLoc, err := types.NewACIdentifier(loc); err != nil {
				return errors.Trace(err)
			} else if err := Host.TrustKey(*acnLoc, "", trustFingerprint); err != nil {
				return errors.Trace(err)
			}
		} else if err := Host.TrustKey(trustPrefix, loc, trustFingerprint); err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func untrustKeys(args []string) error {
	ks := Host.Keystore()
	for _, fprint := range args {
		fmt.Println("Untrusting:", fprint)
		if removed, err := ks.UntrustKey(fprint); err != nil {
			return errors.Trace(err)
		} else {
			fmt.Println("Removed from:", removed)
		}
	}
	return nil
}
