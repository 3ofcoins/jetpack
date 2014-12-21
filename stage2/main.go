package main

import "flag"
import "fmt"
import "os"
import "syscall"

func JailAttach(jid int) error {
	if _, _, err := syscall.Syscall(syscall.SYS_JAIL_ATTACH, uintptr(jid), 0, 0); err == 0 {
		return nil
	} else {
		return err
	}
}

var JID int
var User, Group, AppName string
var Environment = make(dictFlag)
var Exec []string

func main() {
	flag.IntVar(&JID, "jid", -1, "Jail ID")
	flag.StringVar(&User, "user", "root", "User to run as")
	flag.StringVar(&Group, "group", "", "Group to run as")
	flag.StringVar(&AppName, "name", "", "Application name")
	flag.Var(&Environment, "setenv", "Environment variables")

	flag.Parse()
	Exec = flag.Args()

	// TODO: sanity check?

	if err := JailAttach(JID); err != nil {
		panic(err)
	}

	if err := os.Chdir("/"); err != nil {
		panic(err)
	}

	user, err := getUserData(User)
	if err != nil {
		panic(err)
	} else if user == nil {
		panic(fmt.Sprintf("User not found: %s", User))
	}

	var gid int
	if Group == "" {
		gid = user.gid
	} else {
		agid, err := getGid(Group)
		if err != nil {
			panic(err)
		} else if agid < 0 {
			panic(fmt.Sprintf("Group not found: %s", Group))
		}
		gid = agid
	}

	os.Clearenv()

	// Put environment in a map to avoid duplicates when App.Environment
	// overrides one of the default variables

	env := map[string]string{
		"PATH":    "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"USER":    user.username,
		"LOGNAME": user.username,
		"HOME":    user.home,
		"SHELL":   user.shell,
	}

	for k, v := range Environment {
		env[k] = v
	}

	env["AC_APP_NAME"] = AppName
	env["AC_METADATA_URL"] = ""

	envv := make([]string, 0, len(env))
	for k, v := range env {
		envv = append(envv, k+"="+v)
	}

	if err := syscall.Setgroups([]int{}); err != nil {
		panic(err)
	}

	if err := syscall.Setregid(gid, gid); err != nil {
		panic(err)
	}

	if err := syscall.Setreuid(user.uid, user.uid); err != nil {
		panic(err)
	}

	if err := syscall.Exec(Exec[0], Exec, envv); err != nil {
		panic(err)
	}
}
