package main

import "log"

import zj "github.com/3ofcoins/zettajail"

func main() {
	cli := zj.NewCli("")
	cli.StringVar(&zj.ZFSRoot, "root", zj.ZFSRoot, "Root ZFS filesystem")
	// TODO: add commands here
	cli.AddCommand("foo", "-- Do the Foo", func(name string, args []string, sink interface{}) error {
		log.Println("Foo!", name, args, sink)
		return nil
	}, nil)

	cli.Parse(nil)
	if err := cli.Run(); err != nil {
		log.Fatalln(err)
	}
}
