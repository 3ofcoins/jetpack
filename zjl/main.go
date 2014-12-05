package main

import "log"

import zj "github.com/3ofcoins/zettajail"

func main() {
	zj.Cli.Parse(nil)
	if err := zj.Cli.Run(); err != nil {
		log.Fatalln(err)
	}
}
