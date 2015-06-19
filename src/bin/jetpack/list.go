package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/appc/spec/schema/types"
)

func init() {
	AddCommand("images", "List images", cmdListImages, flListImages)
	AddCommand("list", "List pods", cmdListPods, flList)
}

var MachineFriendly, LongHash bool

func flList(fl *flag.FlagSet) {
	QuietFlag(fl, "show only IDs")
	fl.BoolVar(&MachineFriendly, "H", false, "machine-friendly output")
}

func flListImages(fl *flag.FlagSet) {
	flList(fl)
	fl.BoolVar(&LongHash, "l", false, "Show full sha-512 hashes")
}

func cmdListImages([]string) error {
	images := Host.Images()
	items := make([][]string, len(images))
	for i, img := range images {
		items[i] = []string{img.ID(), img.String()}
	}
	return doList("ID\tNAME", items)
}

func cmdListPods([]string) error {
	pods := Host.Pods()
	items := make([][]string, len(pods))
	for i, pod := range pods {
		apps := make([]string, len(pod.Manifest.Apps))
		for j, app := range pod.Manifest.Apps {
			apps[j] = app.Name.String()
		}
		ipAddress, _ := pod.Manifest.Annotations.Get("ip-address")
		items[i] = []string{
			pod.ID(),
			pod.Status().String(),
			ipAddress,
			strings.Join(apps, ", "),
		}
	}
	return doList("ID\tSTATUS\tIP\tAPPS\t", items)
}

func doList(header string, items [][]string) error {
	return doListF(os.Stdout, header, items)
}

func doListF(w io.Writer, header string, items [][]string) error {
	if !LongHash {
		for i := range items {
			items[i][0] = types.ShortHash(items[i][0])
		}
	}

	lines := make([]string, len(items))
	for i, item := range items {
		if Quiet {
			lines[i] = item[0]
		} else {
			lines[i] = strings.Join(item, "\t")
		}
	}
	sort.Strings(lines)

	if !(Quiet || MachineFriendly || header == "") {
		lines = append([]string{header}, lines...)
	}
	output := strings.Join(lines, "\n")

	if MachineFriendly {
		_, err := fmt.Println(output)
		return err
	} else {
		tw := tabwriter.NewWriter(w, 2, 8, 2, ' ', 0)
		fmt.Fprintln(tw, output)
		return tw.Flush()
	}
}
