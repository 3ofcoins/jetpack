package zettajail

import "reflect"
import "testing"

var parsePropertiesCases = []struct {
	in  []string
	out map[string]string
}{
	{nil, nil},
	{[]string{}, map[string]string{}},
	{
		[]string{"interface=lo1", "persist", "mount.devfs", "exec.start=/bin/sh /etc/rc"},
		map[string]string{
			"zettajail:jail:interface":   "\"lo1\"",
			"zettajail:jail:persist":     "true",
			"zettajail:jail:mount.devfs": "true",
			"zettajail:jail:exec.start":  "\"/bin/sh /etc/rc\"",
		},
	},
	{
		[]string{"foo=1", "@bar=2", "+baz=3"},
		map[string]string{
			"zettajail:jail:foo": "\"1\"",
			"zettajail:bar":      "2",
			"baz":                "3",
		},
	},
	{
		[]string{"foo", "nobar", "baz.quux", "baz.noxyzzy", "@fred", "@nobarney", "+barney", "+nofred"},
		map[string]string{
			"zettajail:jail:foo":       "true",
			"zettajail:jail:bar":       "false",
			"zettajail:jail:baz.quux":  "true",
			"zettajail:jail:baz.xyzzy": "false",
			"zettajail:fred":           "on",
			"zettajail:barney":         "off",
			"barney":                   "on",
			"fred":                     "off",
		},
	},
}

func TestParseProperties(t *testing.T) {
	for _, tc := range parsePropertiesCases {
		props := ParseProperties(tc.in)
		if !reflect.DeepEqual(props, tc.out) {
			t.Errorf("parseProperties(%#v) returns %#v, want %#v", tc.in, props, tc.out)
		}
	}
}
