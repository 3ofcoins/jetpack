package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"lib/ui"
)

func main() {
	sw := ui.NewSpinningWriter("", ioutil.Discard)
	defer sw.Close()
	if _, err := io.Copy(sw, os.Stdin); err != nil {
		fmt.Println(err)
	}
}
