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

func (ui *UI) Section(name string, inner func() error) error {
	ui.Sayf("%v:", name)
	ui.Indent("  ")
	defer ui.Dedent()
	return inner()
}

func (ui *UI) Say(what string) {
	for _, line := range strings.Split(what, "\n") {
		ui.out.Write([]byte(ui.indentString + line + "\n"))
	}
}

func (ui *UI) Sayf(format string, args ...interface{}) {
	ui.Say(fmt.Sprintf(format, args...))
}

func (ui *UI) Table(data [][]string) {
	if len(data) == 0 {
		return
	}
	ncol := 0
	for _, row := range data {
		if ncol < len(row) {
			ncol = len(row)
		}
	}
	widths := make([]int, ncol)

	for _, row := range data {
		for j, elt := range row {
			if widths[j] < len(elt) {
				widths[j] = len(elt)
			}
		}
	}

	formatPieces := make([]string, ncol)
	for i, width := range widths {
		formatPieces[i] = fmt.Sprintf("%%-%ds", width)
	}
	format := strings.Join(formatPieces, "  ")

	irow := make([]interface{}, ncol)
	for _, row := range data {
		for i, s := range row {
			irow[i] = s
		}
		for i := len(row); i < ncol; i++ {
			irow[i] = ""
		}
		ui.Say(strings.TrimRight(fmt.Sprintf(format, irow...), " "))
	}
}
