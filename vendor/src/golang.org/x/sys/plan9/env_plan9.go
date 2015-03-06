// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Plan 9 environment variables.

package plan9

import (
	"errors"
	"sync"
)

var (
	// envOnce guards copyenv, which populates env.
	envOnce sync.Once

	// envLock guards env and envs.
	envLock sync.RWMutex

	// env maps from an environment variable to its value.
	env = make(map[string]string)

	// envs contains elements of env in the form "key=value".
	envs []string

	errZeroLengthKey = errors.New("zero length key")
	errShortWrite    = errors.New("i/o count too small")
)

func readenv(key string) (string, error) {
	fd, err := Open("/env/"+key, O_RDONLY)
	if err != nil {
		return "", err
	}
	defer Close(fd)
	l, _ := Seek(fd, 0, 2)
	Seek(fd, 0, 0)
	buf := make([]byte, l)
	n, err := Read(fd, buf)
	if err != nil {
		return "", err
	}
	if n > 0 && buf[n-1] == 0 {
		buf = buf[:n-1]
	}
	return string(buf), nil
}

func writeenv(key, value string) error {
	fd, err := Create("/env/"+key, O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer Close(fd)
	b := []byte(value)
	n, err := Write(fd, b)
	if err != nil {
		return err
	}
	if n != len(b) {
		return errShortWrite
	}
	return nil
}

func copyenv() {
	fd, err := Open("/env", O_RDONLY)
	if err != nil {
		return
	}
	defer Close(fd)
	files, err := readdirnames(fd)
	if err != nil {
		return
	}
	envs = make([]string, len(files))
	i := 0
	for _, key := range files {
		v, err := readenv(key)
		if err != nil {
			continue
		}
		env[key] = v
		envs[i] = key + "=" + v
		i++
	}
}

// readdirnames returns the names of files inside the directory represented by dirfd.
func readdirnames(dirfd int) (names []string, err error) {
	names = make([]string, 0, 100)
	var buf [STATMAX]byte

	for {
		n, e := Read(dirfd, buf[:])
		if e != nil {
			return nil, e
		}
		if n == 0 {
			break
		}
		for i := 0; i < n; {
			m, _ := gbit16(buf[i:])
			m += 2

			if m < STATFIXLEN {
				return nil, ErrBadStat
			}

			s, _, ok := gstring(buf[i+41:])
			if !ok {
				return nil, ErrBadStat
			}
			names = append(names, s)
			i += int(m)
		}
	}
	return
}

func Getenv(key string) (value string, found bool) {
	if len(key) == 0 {
		return "", false
	}

	envLock.RLock()
	defer envLock.RUnlock()

	if v, ok := env[key]; ok {
		return v, true
	}
	v, err := readenv(key)
	if err != nil {
		return "", false
	}
	env[key] = v
	envs = append(envs, key+"="+v)
	return v, true
}

func Setenv(key, value string) error {
	if len(key) == 0 {
		return errZeroLengthKey
	}

	envLock.Lock()
	defer envLock.Unlock()

	err := writeenv(key, value)
	if err != nil {
		return err
	}
	env[key] = value
	envs = append(envs, key+"="+value)
	return nil
}

func Clearenv() {
	envLock.Lock()
	defer envLock.Unlock()

	env = make(map[string]string)
	envs = []string{}
	RawSyscall(SYS_RFORK, RFCENVG, 0, 0)
}

func Environ() []string {
	envLock.RLock()
	defer envLock.RUnlock()

	envOnce.Do(copyenv)
	return append([]string(nil), envs...)
}
