package main

import "log"

import zj "github.com/3ofcoins/zettajail"

func main() {
	cli := zj.NewCli("")
	cli.StringVar(&zj.ZFSRoot, "root", zj.ZFSRoot, "Root ZFS filesystem")
	for _, cmd := range zj.Commands {
		cli.AddCommand(cmd)
	}
	cli.Parse(nil)
	if err := cli.Run(); err != nil {
		log.Fatalln(err)
	}
}
