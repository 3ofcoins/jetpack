package main

import "fmt"

// FIXME: THE WHOLE THING

func init() {
	AddCommand("mds", "Check metadata service process", cmdMds, nil)
}

func cmdMds(args []string) error {
	if mdsi, err := Host.CheckMDS(); err != nil && mdsi == nil {
		fmt.Println("Could not find metadata service:", err)
		return err
	} else if err != nil {
		fmt.Printf("Metadata service ERROR (%v): %v\n", err, mdsi)
		return err
	} else {
		fmt.Println("Metadata service OK:", mdsi)
		return nil
	}
}
