package config

import "errors"
import "fmt"

var ErrKeyNotFound = errors.New("Config key not found")

type ErrInvalidValue struct {
	typ string
	Val string
	Err error
}

func invalidValue(typ, val string, err error) *ErrInvalidValue {
	return &ErrInvalidValue{typ, val, err}
}

func (err *ErrInvalidValue) Error() string {
	rv := fmt.Sprintf("Invalid %v value %#v", err.typ, err.Val)
	if err.Err != nil {
		rv += ": " + err.Err.Error()
	}
	return rv
}

func (err *ErrInvalidValue) String() string {
	return err.Error()
}

func IsInvalidValue(err error) bool {
	_, ok := err.(*ErrInvalidValue)
	return ok
}
