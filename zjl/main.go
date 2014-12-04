package main

import "log"

import "github.com/3ofcoins/zettajail"

func main() {
	if err := zettajail.RunCli(); err != nil {
		log.Fatalln(err)
	}
}
