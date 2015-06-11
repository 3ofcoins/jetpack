package size

import (
	"testing"
)

var capacityTests = []struct {
	str string
	c   Capacity
}{
	{"0", 0},
	{"1K", 1024},
	{"1K", 1025}, // Rounding down
	{"1.1K", 1090},
	{"2K", 2047}, // Rounding up
	{"135.2M", 141798563},
	{"1M", 1 * Megabyte},
	{"2G", 2 * Gigabyte},
	{"1T", 1 * Terabyte},
	{"1P", 1 * Petabyte},
	{"1E", 1 * Exabyte},
}

func TestCapacityString(t *testing.T) {
	for _, tt := range capacityTests {
		if str := tt.c.String(); str != tt.str {
			t.Errorf("Capacity(%d).String() = %s, want %s", uint64(tt.c), str, tt.str)
		}
	}
}

var parseTests = []struct {
	str string
	c   Capacity
}{
	{"0", 0},
	{"100", 100},
	{"1K", 1 * Kilobyte},
	{"1M", 1 * Megabyte},
	{"2G", 2 * Gigabyte},
	{"1T", 1 * Terabyte},
	{"1P", 1 * Petabyte},
	{"1E", 1 * Exabyte},
	{"100M", 100 * Megabyte},
}

func TestParseCapacity(t *testing.T) {
	for _, tt := range parseTests {
		if c, err := ParseCapacity(tt.str); c != tt.c || err != nil {
			t.Errorf("ParseCapacity(%s) = %d, want %d, err = %s", tt.str, c, tt.c, err)
		}
	}
}

func TestSetCapacity(t *testing.T) {
	var c Capacity
	for _, tt := range parseTests {
		if err := (&c).Set(tt.str); c != tt.c || err != nil {
			t.Errorf("Set(%s) = %d, want %d, err = %s", tt.str, c, tt.c, err)
		}
	}
}
