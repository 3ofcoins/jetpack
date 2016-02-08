package main

import (
	"os/exec"
	"strings"

	"github.com/3ofcoins/jetpack/lib/acutil"
	"github.com/appc/spec/schema/types"
)

type ListedImage struct {
	Hash   types.Hash
	Name   types.ACIdentifier
	Labels types.Labels
}

func listImages(args ...string) []ListedImage {
	out, err := exec.Command("jetpack", "images", "-H", "-l").Output()
	if err != nil {
		panic(err)
	}

	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	rv := make([]ListedImage, len(lines))

	for i, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			panic("Invalid `jetpack images` output line: " + line)
		}
		hash, err := types.NewHash(fields[0])
		if err != nil {
			panic(err)
		}

		name, labels, err := acutil.ParseImageName(fields[1])
		if err != nil {
			panic(err)
		}

		rv[i] = ListedImage{
			Hash:   *hash,
			Name:   name,
			Labels: labels,
		}
	}

	return rv
}

func isThereImage(name string) (bool, error) {
	acid, labels, err := acutil.ParseImageName(imgName)
	if err != nil {
		return false, err
	}

	for _, img := range listImages() {
		if acid.Equals(img.Name) && acutil.MatchLabels(labels, img.Labels) {
			return true, nil
		}
	}
	return false, nil
}
