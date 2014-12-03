package main

import "log"

func main() {
	if err := RunCli(); err != nil {
		log.Fatalln(err)
	}
}
