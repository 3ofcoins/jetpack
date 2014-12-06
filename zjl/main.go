package main

import "log"

import zj "github.com/3ofcoins/zettajail"

func main() {
	if err := zj.RunCli("", nil); err != nil {
		log.Fatalln(err)
	}
}
