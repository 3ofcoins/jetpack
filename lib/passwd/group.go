package passwd

import (
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/juju/errors"
)

type GroupEntry struct {
	Name string
	Gid  int
}

type GroupFile []GroupEntry

func ReadGroup(path string) (GroupFile, error) {
	if content, err := ioutil.ReadFile(path); os.IsNotExist(err) {
		// No group file is same as empty group file
		return GroupFile{}, nil
	} else if err != nil {
		return nil, errors.Trace(err)
	} else {
		var rv GroupFile
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line[0] == '#' {
				continue
			}
			fields := strings.Split(line, ":")
			gid, err := strconv.Atoi(fields[2])
			if err != nil {
				return nil, errors.Trace(err)
			}
			rv = append(rv, GroupEntry{
				Name: fields[0],
				Gid:  gid,
			})
		}
		return rv, nil
	}
}

func (gf GroupFile) FindByName(name string) *GroupEntry {
	for _, entry := range gf {
		if entry.Name == name {
			return &entry
		}
	}
	return nil
}

func (gf GroupFile) FindGid(spec string) int {
	if grent := gf.FindByName(spec); grent != nil {
		return grent.Gid
	} else if gid, err := strconv.Atoi(spec); err != nil {
		return -1
	} else {
		return gid
	}
}
