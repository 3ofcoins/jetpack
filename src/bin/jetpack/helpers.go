package main

import "fmt"

type sliceFlag []string

func (sf *sliceFlag) String() string {
	return fmt.Sprintf("%v", *sf)
}

func (sf *sliceFlag) Set(v string) error {
	*sf = append(*sf, v)
	return nil
}
