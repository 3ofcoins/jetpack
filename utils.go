package main

import "log"
import "os/exec"
import "strconv"
import "strings"

func Jls() map[string]int {
	jails := make(map[string]int)
	cmd := exec.Command("jls", "name", "jid")
	out, err := cmd.Output()
	if err != nil {
		log.Fatalln("ERROR:", err)
	}
	for _, ln := range strings.Split(string(out), "\n") {
		if ln == "" {
			continue
		}
		fields := strings.Fields(ln)
		if len(fields) != 2 {
			log.Printf("WTF %#v -> %#v\n", ln, fields)
			continue
		}
		jid, err := strconv.Atoi(fields[1])
		if err != nil {
			log.Fatalf("ERROR parsing %#v: %v\n", ln, err)
		}
		jails[fields[0]] = jid
	}
	return jails
}
