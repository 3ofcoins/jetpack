package ui

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

var Debug bool
var projectRootLength int

func init() {
	if _, file, _, ok := runtime.Caller(0); ok {
		projectRootLength = len(file) - len("lib/ui/ui.go")
	}
}

type UI struct {
	prefix string
}

func NewUI(format string, args ...interface{}) *UI {
	return &UI{fmt.Sprintf(format, args...)}
}

func (ui *UI) Child(format string, args ...interface{}) *UI {
	return NewUI(
		fmt.Sprintf("%%v»%v", format),
		append([]interface{}{ui.prefix}, args...)...,
	)
}

func (ui *UI) utter(what string) {
	if what[len(what)-1] == '\n' {
		what = what[:len(what)-1]
	}
	pos := ""
	if Debug {
		if _, file, line, ok := runtime.Caller(2); ok {
			pos = fmt.Sprintf(" [%v:%v]", file[projectRootLength:], line)
		}
	}
	// TODO: split lines?
	fmt.Fprintf(os.Stderr, "%v %v: %v%v\n",
		time.Now().Format(time.RFC3339),
		ui.prefix, what, pos)
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
