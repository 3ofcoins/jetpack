package ui

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"golang.org/x/crypto/ssh/terminal"

	// "github.com/briandowns/spinner"
	"github.com/mgutz/ansi"
)

var Debug, IsTerminal bool
var projectRootLength int

// TODO: detect UTF-8 environment
const UTF8 = true

var Styles = map[string]string{
	"timestamp":  "black+h",
	"kind":       "blue",
	"separator":  "blue+b",
	"id":         "blue+h",
	"debugInfo":  "black+hu",
	"debugPunct": "black+h",
}

func init() {
	if _, file, _, ok := runtime.Caller(0); ok {
		projectRootLength = len(file) - len("lib/ui/ui.go")
	}
	IsTerminal = terminal.IsTerminal(2)
	if !IsTerminal {
		ansi.DisableColors(true)
	}
	Init()
}

var debugFormat string

func Init() {
	debugFormat = strings.Join([]string{
		ansi.Color("[", Styles["debugPunct"]),
		ansi.Color("%v", Styles["debugInfo"]),
		ansi.Color(":", Styles["debugPunct"]),
		ansi.Color("%v", Styles["debugInfo"]),
		ansi.Color("]", Styles["debugPunct"]),
	}, "")
}

type UI struct {
	kind, id, format string
}

func uiFormat(color, kind, id string) string {
	if id == "" {
		return fmt.Sprintf("%v %v %%v",
			ansi.Color("%v", Styles["timestamp"]),
			ansi.Color(kind, color))
	} else {
		return fmt.Sprintf("%v %v%v %%v",
			ansi.Color("%v", Styles["timestamp"]),
			ansi.Color(kind+":", color),
			ansi.Color(id, color+"+h"))
	}
}

func NewUI(color, kind, id string) *UI {
	return &UI{kind, id, uiFormat(color, kind, id)}
}

func (ui *UI) utter(what string) {
	if what[len(what)-1] == '\n' {
		what = what[:len(what)-1]
	}
	ts := time.Now().Format(time.RFC3339)
	ln := fmt.Sprintf(ui.format, ts, what)
	if Debug {
		_, file, line, ok := runtime.Caller(2)
		if !ok {
			file = "?"
			line = -1
		} else {
			file = file[projectRootLength:]
		}
		fmt.Fprintln(os.Stderr, ln, fmt.Sprintf(debugFormat, file, line))
	} else {
		fmt.Fprintln(os.Stderr, ln)
	}
}

func (ui *UI) Println(args ...interface{}) {
	ui.utter(fmt.Sprintln(args...))
}

func (ui *UI) Printf(format string, args ...interface{}) {
	ui.utter(fmt.Sprintf(format, args...))
}

func (ui *UI) Debug(args ...interface{}) {
	if Debug {
		ui.utter("DEBUG: " + fmt.Sprintln(args...))
	}
}

func (ui *UI) Debugf(format string, args ...interface{}) {
	if Debug {
		ui.utter("DEBUG: " + fmt.Sprintf(format, args...))
	}
}
