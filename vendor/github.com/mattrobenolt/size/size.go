// Package size implements functionality for working with byte sizes.
package size

import (
	"errors"
	"strconv"
)

// A Capacity represents a size in bytes.
type Capacity uint64

// Common capacities.
//
// To count the number of units in a Capacity, divide:
//	gigabyte := size.Gigabyte
//	fmt.Print(gigabyte/size.Kilobyte) // prints 1048576
//
// To convert an integer number of units to a Capacity, multiply:
//	gigabytes := 5
//	fmt.Print(gigabytes*size.Gigabyte) // prints 5G
const (
	Byte     Capacity = 1
	Kilobyte          = Byte << 10
	Megabyte          = Kilobyte << 10
	Gigabyte          = Megabyte << 10
	Terabyte          = Gigabyte << 10
	Petabyte          = Terabyte << 10
	Exabyte           = Petabyte << 10
)

// Bytes returns the capacity as an integer bytes count.
func (c Capacity) Bytes() uint64 { return uint64(c) }

// Kilobytes returns the capacity as an integer kilobytes count.
func (c Capacity) Kilobytes() uint64 { return c.Bytes() >> 10 }

// Megabytes returns the capacity as an integer megabytes count.
func (c Capacity) Megabytes() uint64 { return c.Kilobytes() >> 10 }

// Gigabytes returns the capacity as an integer gigabytes count.
func (c Capacity) Gigabytes() uint64 { return c.Megabytes() >> 10 }

// Terabytes returns the capacity as an integer terabytes count.
func (c Capacity) Terabytes() uint64 { return c.Gigabytes() >> 10 }

// Petabytes returns the capacity as an integer petabytes count.
func (c Capacity) Petabytes() uint64 { return c.Terabytes() >> 10 }

// Exabytes returns the capacity as an integer exabytes count.
func (c Capacity) Exabytes() uint64 { return c.Petabytes() >> 10 }

var units = [...]struct {
	Suffix byte
	Size   uint64
}{
	{'E', uint64(Exabyte)},
	{'P', uint64(Petabyte)},
	{'T', uint64(Terabyte)},
	{'G', uint64(Gigabyte)},
	{'M', uint64(Megabyte)},
	{'K', uint64(Kilobyte)},
}

var unitMap = map[byte]Capacity{
	'E': Exabyte,
	'P': Petabyte,
	'T': Terabyte,
	'G': Gigabyte,
	'M': Megabyte,
	'K': Kilobyte,
}

// String returns a string representing the capacity in the form of "10.4G".
// Capacities are rounded to the nearest 1/10th within the largest
// unit of granularity. This format is similar to the "human" output from
// common Linux tools.
func (c Capacity) String() string {
	u := uint64(c)

	// If we're less than a kilobyte, we just display the number without a unit
	if u < uint64(Kilobyte) {
		if u == 0 {
			return "0"
		}
		return strconv.FormatUint(u, 10)
	}

	// Longest we could have is 1023.1P
	var buf [7]byte
	w := len(buf)

	// Find the largest unit that we can fit into
	for _, unit := range units {
		if u == unit.Size {
			w -= 2
			buf[w] = '1'
			buf[w+1] = unit.Suffix
			break
		} else if u > unit.Size {
			w--
			buf[w] = unit.Suffix
			w, u = fmtFrac(buf[:w], u, unit.Size)
			w = fmtInt(buf[:w], u)
			break
		}
	}

	return string(buf[w:])
}

// Set parses the given string and sets the receiver to the parsed value.
func (c *Capacity) Set(s string) error {
	v, err := ParseCapacity(s)
	if err != nil {
		return err
	}
	*c = v
	return nil
}

// fmtFrac formats the fraction of v/unit (e.g., ".1") into the
// tail of buf, omitting trailing zeros. It omits the decimal
// point too when the fraction is 0. It returns the index where the
// output bytes begin and the value v/unit.
func fmtFrac(buf []byte, v, unit uint64) (nw int, nv uint64) {
	w := len(buf)
	frac := (v % unit * 100) / unit
	v /= unit

	// Just round down
	if frac < 5 {
		return w, v
	}
	// Rouding up to the next whole unit
	if frac >= 95 {
		return w, v + 1
	}

	w -= 2
	buf[w] = '.'
	buf[w+1] = byte((frac+5)/10) + '0'
	return w, v
}

// fmtInt formats v into the tail of buf.
// It returns the index where the output begins.
func fmtInt(buf []byte, v uint64) int {
	w := len(buf)
	if v == 0 {
		w--
		buf[w] = '0'
	} else {
		for v > 0 {
			w--
			buf[w] = byte(v%10) + '0'
			v /= 10
		}
	}
	return w
}

// ParseCapacity parses a capacity string.
// A capacity string may only contain whole integers and one unit suffix,
// such as "10G" or "5T".
// Valid capacity units are "K", "M", "G", "T", "P", "E".
func ParseCapacity(s string) (Capacity, error) {
	if s == "" {
		return 0, errors.New("size: empty capacity")
	}
	if s == "0" {
		return 0, nil
	}

	var c uint64
	orig := s
	i := 0
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			break
		}
		c = c*10 + uint64(s[i]) - '0'
	}
	s = s[i:]

	if s == "" {
		return Capacity(c), nil
	}
	if len(s) != 1 {
		return 0, errors.New("size: invalid capacity " + orig + " " + s)
	}
	unit, ok := unitMap[s[0]]
	if !ok {
		return 0, errors.New("size: invalid capacity " + orig)
	}
	return Capacity(c) * unit, nil
}
