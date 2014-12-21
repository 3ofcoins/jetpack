//+build cgo

package main

/*
#include <errno.h>
#include <stdlib.h>
#include <pwd.h>
#include <grp.h>
*/
import "C"

import "unsafe"

func getGid(groupname string) (int, error) {
	cgroupname := C.CString(groupname)
	defer C.free(unsafe.Pointer(cgroupname))
	v, err := C.getgrnam(cgroupname)
	gr := (*C.struct_group)(v)
	if gr == nil {
		return -1, err
	} else {
		return int(gr.gr_gid), nil
	}
}

type userData struct {
	username    string
	uid, gid    int
	home, shell string
}

func getUserData(username string) (*userData, error) {
	cusername := C.CString(username)
	defer C.free(unsafe.Pointer(cusername))
	v, err := C.getpwnam(cusername)
	pw := (*C.struct_passwd)(v)
	if pw == nil {
		return nil, err
	} else {
		return &userData{
			C.GoString(pw.pw_name),
			int(pw.pw_uid),
			int(pw.pw_gid),
			C.GoString(pw.pw_dir),
			C.GoString(pw.pw_shell),
		}, nil
	}
}
