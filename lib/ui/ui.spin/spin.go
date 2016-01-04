package main

import (
	"github.com/3ofcoins/jetpack/lib/ui"
	"time"
)

func main() {
	s := ui.NewSpinner("Waiting...", ui.SuffixElapsed(), nil)
	for i := 0; i < 32; i++ {
		s.Step()
		time.Sleep(250 * time.Millisecond)
	}
	s.Finish()
}
