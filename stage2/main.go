package main

import "flag"
import "io/ioutil"
import "os"
import "strconv"
import "strings"
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
var WorkingDirectory string

func main() {
	flag.IntVar(&JID, "jid", -1, "Jail ID")
	flag.StringVar(&User, "user", "root", "User to run as")
	flag.StringVar(&Group, "group", "", "Group to run as")
	flag.StringVar(&AppName, "name", "", "Application name")
	flag.StringVar(&WorkingDirectory, "cwd", "/", "Working directory")
	flag.Var(&Environment, "setenv", "Environment variables")

	flag.Parse()
	Exec = flag.Args()

	// TODO: sanity check?

	if err := JailAttach(JID); err != nil {
		panic(err)
	}

	if err := os.Chdir(WorkingDirectory); err != nil {
		panic(err)
	}

	Uid := -1
	Gid := -1
	var Username, Home, Shell string

	if bytes, err := ioutil.ReadFile("/etc/passwd"); err != nil {
		panic(err)
	} else {
		for _, line := range strings.Split(string(bytes), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line[0] == '#' {
				continue
			}
			fields := strings.Split(line, ":")
			if fields[0] == User || fields[2] == User {
				if uid, err := strconv.Atoi(fields[2]); err != nil {
					panic(err)
				} else {
					Uid = uid
				}
				if gid, err := strconv.Atoi(fields[3]); err != nil {
					panic(err)
				} else {
					Gid = gid
				}
				Username = fields[0]
				Home = fields[5]
				Shell = fields[6]
				break
			}
		}
	}

	if Uid < 0 {
		panic("User not found")
	}

	if Group != "" {
		Gid = -1
		if gid, err := strconv.Atoi(Group); err != nil {
			if bytes, err := ioutil.ReadFile("/etc/group"); err != nil {
				panic(err)
			} else {
				for _, line := range strings.Split(string(bytes), "\n") {
					line = strings.TrimSpace(line)
					if line == "" || line[0] == '#' {
						continue
					}
					fields := strings.Split(line, ":")
					if fields[0] == Group {
						if gid, err := strconv.Atoi(fields[2]); err != nil {
							panic(err)
						} else {
							Gid = gid
						}
						break
					}
				}
			}
		} else {
			Gid = gid
		}
		if Gid < 0 {
			panic("Group not found")
		}
	}

	os.Clearenv()

	// Put environment in a map to avoid duplicates when App.Environment
	// overrides one of the default variables

	env := map[string]string{
		"PATH":    "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"USER":    Username,
		"LOGNAME": Username,
		"HOME":    Home,
		"SHELL":   Shell,
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

	if err := syscall.Setregid(Gid, Gid); err != nil {
		panic(err)
	}

	if err := syscall.Setreuid(Uid, Uid); err != nil {
		panic(err)
	}

	// FIXME: setusercontext()?
	// See https://github.com/freebsd/freebsd/blob/master/usr.sbin/jexec/jexec.c#L123-L126

	if err := syscall.Exec(Exec[0], Exec, envv); err != nil {
		panic(err)
	}
}
