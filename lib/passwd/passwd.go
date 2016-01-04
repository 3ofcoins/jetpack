package passwd

import (
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/juju/errors"
)

type PasswdEntry struct {
	Username    string
	Uid, Gid    int
	Home, Shell string
}

type PasswdFile []PasswdEntry

func ReadPasswd(path string) (PasswdFile, error) {
	if content, err := ioutil.ReadFile(path); os.IsNotExist(err) {
		// No passwd file is same as empty passwd file
		return PasswdFile{}, nil
	} else if err != nil {
		return nil, errors.Trace(err)
	} else {
		var rv PasswdFile
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line[0] == '#' {
				continue
			}
			fields := strings.Split(line, ":")
			uid, err := strconv.Atoi(fields[2])
			if err != nil {
				return nil, errors.Trace(err)
			}
			gid, err := strconv.Atoi(fields[3])
			if err != nil {
				return nil, errors.Trace(err)
			}
			rv = append(rv, PasswdEntry{
				Username: fields[0],
				Uid:      uid,
				Gid:      gid,
				Home:     fields[5],
				Shell:    fields[6],
			})
		}
		return rv, nil
	}
}

func (pwf PasswdFile) FindByUsername(username string) *PasswdEntry {
	for _, entry := range pwf {
		if entry.Username == username {
			return &entry
		}
	}
	return nil
}

func (pwf PasswdFile) FindByUid(uid int) *PasswdEntry {
	for _, entry := range pwf {
		if entry.Uid == uid {
			return &entry
		}
	}
	return nil
}

func (pwf PasswdFile) Find(spec string) *PasswdEntry {
	if spec == "" {
		if pwent := pwf.FindByUid(0); pwent != nil {
			return pwent
		} else {
			return &PasswdEntry{
				Username: "root",
				Uid:      0,
				Gid:      0,
				Home:     "/root",
				Shell:    "/bin/sh",
			}
		}
	}
	if pwent := pwf.FindByUsername(spec); pwent != nil {
		return pwent
	}
	if uid, err := strconv.Atoi(spec); err == nil {
		if pwent := pwf.FindByUid(uid); pwent != nil {
			return pwent
		} else if uid == 0 {
			return &PasswdEntry{
				Username: "root",
				Uid:      0,
				Gid:      0,
				Home:     "/root",
				Shell:    "/bin/sh",
			}
		} else {
			return &PasswdEntry{
				Username: "",
				Uid:      uid,
				Gid:      -1,
				Home:     "/",
				Shell:    "/bin/sh",
			}
		}
	}
	return nil
}
