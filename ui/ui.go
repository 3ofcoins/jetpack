package ui

import "fmt"
import "io"
import "strings"

// import "github.com/mgutz/ansi"

type UI struct {
	out          io.Writer
	indentStack  []string
	indentString string
}

func NewUI(out io.Writer) *UI {
	return &UI{out: out}
}

func (ui *UI) Indent(prefix string) {
	ui.indentStack = append(ui.indentStack, prefix)
	ui.indentString = strings.Join(ui.indentStack, "")
}

func (ui *UI) Indentf(format string, args ...interface{}) {
	ui.Indent(fmt.Sprintf(format, args...))
}

func (ui *UI) Dedent() {
	if len(ui.indentStack) > 0 {
		ui.indentStack = ui.indentStack[:len(ui.indentStack)-1]
	}
	ui.indentString = strings.Join(ui.indentStack, "")
}

func (ui *UI) IsIndented() bool {
	return len(ui.indentStack) > 0
}

func (ui *UI) Say(what string) {
	for _, line := range strings.Split(what, "\n") {
		ui.out.Write([]byte(ui.indentString + line + "\n"))
	}
}

func (ui *UI) Sayf(format string, args ...interface{}) {
	ui.Say(fmt.Sprintf(format, args...))
}
