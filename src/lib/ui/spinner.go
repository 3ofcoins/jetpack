package ui

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/mattrobenolt/size"
)

// http://ascii-table.com/ansi-escape-sequences-vt-100.php

// Clear text until EOL
const EL0 = "\033[K"

var UnicodeSteps = []string{"⠿", "⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
var AsciiSteps = []string{"*", "/", "-", "\\", "|"}

type Spinner struct {
	prefix string
	suffix func() string
	step   int
	steps  []string
}

func NewSpinner(prefix string, suffixCallback func() string, steps []string) *Spinner {
	if prefix != "" {
		prefix = prefix + " "
	}
	if steps == nil {
		if UTF8 {
			steps = UnicodeSteps
		} else {
			steps = AsciiSteps
		}
	}
	return &Spinner{prefix: prefix, suffix: suffixCallback, step: -1, steps: steps}
}

func (s *Spinner) line() string {
	str := "\r" + s.prefix + s.steps[1+s.step] + EL0
	if s.suffix != nil {
		str += " " + s.suffix()
	}
	return str
}

func (s *Spinner) Step() {
	if IsTerminal {
		s.step = (s.step + 1) % (len(s.steps) - 1)
		fmt.Fprint(os.Stderr, s.line())
	}
}

func (s *Spinner) Finish() {
	if s.step >= 0 || !IsTerminal {
		s.step = -1
		fmt.Fprintln(os.Stderr, s.line())
	}
}

const elapsedResolution = time.Millisecond

type ElapsedSince time.Time

func Elapsed() ElapsedSince {
	return ElapsedSince(time.Now())
}

func (e ElapsedSince) Duration() time.Duration {
	return time.Now().Sub(time.Time(e))
}

func (e ElapsedSince) String() string {
	d := e.Duration()
	d /= elapsedResolution
	d *= elapsedResolution
	return d.String()
}

func SuffixElapsed() func() string {
	return Elapsed().String
}

type SpinningWriter struct {
	spinner *Spinner
	ticker  *time.Ticker
	wrote   size.Capacity
	w       io.Writer
	ela     ElapsedSince
	stop    chan bool
}

func NewSpinningWriter(desc string, w io.Writer) *SpinningWriter {
	if w == nil {
		w = ioutil.Discard
	}
	sw := &SpinningWriter{
		w:      w,
		ela:    Elapsed(),
		ticker: time.NewTicker(250 * time.Millisecond),
		stop:   make(chan bool, 1),
	}
	sw.spinner = NewSpinner(desc, sw.String, nil)
	go sw.spin()
	return sw
}

func (sw *SpinningWriter) spin() {
	for {
		select {
		case <-sw.ticker.C:
			sw.spinner.Step()
		case <-sw.stop:
			sw.spinner.Finish()
			sw.stop <- true
			return
		}
	}
}

func (sw *SpinningWriter) Write(p []byte) (n int, err error) {
	defer func() { sw.wrote += size.Capacity(n) }()
	return sw.w.Write(p)
}

func (sw *SpinningWriter) Close() error {
	sw.ticker.Stop()
	sw.stop <- true
	<-sw.stop // wait for confirmation
	return nil
}

func (sw *SpinningWriter) String() string {
	return fmt.Sprintf("%v\t%v\t%v/s", sw.wrote, sw.ela, size.Capacity(float64(sw.wrote)/sw.ela.Duration().Seconds()))
}
