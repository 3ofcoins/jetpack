package main

import "fmt"
import "strings"

type dictFlag map[string]string

func (df *dictFlag) Set(val string) error {
	if splut := strings.SplitN(val, "=", 2); len(splut) == 1 {
		(*df)[splut[0]] = ""
	} else {
		(*df)[splut[0]] = splut[1]
	}
	return nil
}

func (df *dictFlag) String() string {
	return fmt.Sprintf("%v", *df)
}
