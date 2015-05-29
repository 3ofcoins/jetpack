package run

import "testing"

import "strings"

func TestIsShellSafe(t *testing.T) {
	for input, expected := range map[string]bool{
		"foo":                   true,
		"-bar":                  true,
		"gżegżółka":             false,
		"foo bar":               false,
		"foo;bar":               false,
		"O'Rety":                false,
		"baz=./quux+XYZZY@1234": true,
	} {
		if actual := IsShellSafe(input); actual != expected {
			t.Logf("expected IsShellSafe(%#v) to be %v, got %v instead", input, expected, actual)
			t.Fail()
		}
	}
}

var sampleQuotations = map[string]string{
	"foo":           "foo",
	"foo bar":       "'foo bar'",
	"foo;bar":       "'foo;bar'",
	"Wielki O'Rety": "'Wielki O'\\''Rety'",
}

func TestShellEscapeWord(t *testing.T) {
	for input, expected := range sampleQuotations {
		if actual := ShellEscapeWord(input); actual != expected {
			t.Logf("expected ShellEscapeWord(%#v) to be %#v, got %#v instead", input, expected, actual)
			t.Fail()
		}
	}
}

func TestShellEscape(t *testing.T) {
	input := make([]string, 0, len(sampleQuotations))
	expectedPieces := make([]string, 0, len(sampleQuotations))
	for unescaped, escaped := range sampleQuotations {
		input = append(input, unescaped)
		expectedPieces = append(expectedPieces, escaped)
	}
	expected := strings.Join(expectedPieces, " ")
	if actual := ShellEscape(input...); actual != expected {
		t.Logf("expected ShellEscape(%#v...) to be %#v, got %#v instead", input, expected, actual)
		t.Fail()
	}
}
