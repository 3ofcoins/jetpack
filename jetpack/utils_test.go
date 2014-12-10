package jetpack

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
			"jetpack:jail:interface":   "\"lo1\"",
			"jetpack:jail:persist":     "true",
			"jetpack:jail:mount.devfs": "true",
			"jetpack:jail:exec.start":  "\"/bin/sh /etc/rc\"",
		},
	},
	{
		[]string{"foo=1", "@bar=2", "+baz=3"},
		map[string]string{
			"jetpack:jail:foo": "\"1\"",
			"jetpack:bar":      "2",
			"baz":                "3",
		},
	},
	{
		[]string{"foo", "nobar", "baz.quux", "baz.noxyzzy", "@fred", "@nobarney", "+barney", "+nofred"},
		map[string]string{
			"jetpack:jail:foo":       "true",
			"jetpack:jail:bar":       "false",
			"jetpack:jail:baz.quux":  "true",
			"jetpack:jail:baz.xyzzy": "false",
			"jetpack:fred":           "on",
			"jetpack:barney":         "off",
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
