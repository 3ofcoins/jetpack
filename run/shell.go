package run

import "strings"

var shellSafe map[rune]bool

func init() {
	shellSafe = make(map[rune]bool)
	for _, rune := range "-%+,./0123456789:=@ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz" {
		shellSafe[rune] = true
	}
}

// Returns true if str is shell-safe
func IsShellSafe(str string) bool {
	return strings.IndexFunc(str, func(r rune) bool { return !shellSafe[r] }) < 0
}

func ShellEscapeWord(str string) string {
	if IsShellSafe(str) {
		return str
	} else {
		return "'" + strings.Replace(str, "'", "'\\''", -1) + "'"
	}
}

func ShellEscape(strs ...string) string {
	for i, str := range strs {
		strs[i] = ShellEscapeWord(str)
	}
	return strings.Join(strs, " ")
}
