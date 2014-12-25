package config

import "testing"

func TestLoadArguments_Flags(t *testing.T) {
	c := NewConfig()
	c.LoadArguments(
		"foo",
		"nobar",
		"+yay",
		"-nay",
		"+flinstones/fred",
		"-flinstones/barney",
		"flinstones/wilma",
		"flinstones/nobetty",
	)

	for name, expected := range map[string]bool{
		"foo":               true,
		"bar":               false,
		"yay":               true,
		"nay":               false,
		"flinstones/fred":   true,
		"flinstones/barney": false,
		"flinstones/wilma":  true,
		"flinstones/betty":  false,
	} {
		if actual, err := c.GetBool(name); err != nil {
			t.Errorf("c.GetBool(%#v) -> %v", name, err)
		} else {
			if actual != expected {
				t.Logf("Expected c.GetBool(%#v) to be %v, got %v instead\n", name, expected, actual)
				t.Fail()
			}
		}
	}
}

func TestDelete(t *testing.T) {
	c := NewConfig()
	c["foo"] = "bar"
	c["baz"] = "quux"

	c.Delete("baz")

	if v, err := c.GetString("foo"); err != nil || v != "bar" {
		t.Logf("Expected c.GetString(\"foo\") to return (\"bar\", nil), got (%#v, %#v) instead", v, err)
		t.Fail()
	}

	if v, err := c.GetString("bar"); err != ErrKeyNotFound {
		t.Logf("Expected c.GetString(\"bar\") to return (\"\", ErrKeyNotFound), got (%#v, %#v) instead", v, err)
		t.Fail()
	}
}
