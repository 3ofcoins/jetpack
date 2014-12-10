package main

import "log"

import "github.com/3ofcoins/jetpack/jetpack"

func main() {
	if err := jetpack.Run("", nil); err != nil {
		log.Fatalln(err)
	}
}
