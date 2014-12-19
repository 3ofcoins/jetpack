package main

import "fmt"
import "os"
import "github.com/juju/errors"
import "github.com/3ofcoins/jetpack/jetpack"

func main() {
	if err := jetpack.Run("", nil); err != nil {
		fmt.Fprintln(os.Stderr, errors.ErrorStack(err))
		os.Exit(1)
	}
}
