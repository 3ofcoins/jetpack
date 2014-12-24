package run

import "fmt"
import "io"
import "os"
import "os/exec"
import "strings"

type Cmd struct {
	Cmd exec.Cmd
}

func (c *Cmd) String() string {
	return fmt.Sprintf("run.Command[%s]", ShellEscape(c.Cmd.Args...))
}

type CmdError struct {
	ExecError error
	Cmd       *Cmd
}

func (err *CmdError) Error() string {
	return fmt.Sprintf("%v: %v", err.Cmd, err.ExecError.Error())
}

func Command(command string, args ...string) *Cmd {
	c := &Cmd{*exec.Command(command, args...)}
	c.Cmd.Stdin = os.Stdin
	c.Cmd.Stdout = os.Stdout
	c.Cmd.Stderr = os.Stderr
	return c
}

func (c *Cmd) wrapError(err error) error {
	if err == nil {
		return nil
	}
	return &CmdError{ExecError: err, Cmd: c}
}

func (c *Cmd) Run() error {
	return c.wrapError(c.Cmd.Run())
}

func (c *Cmd) Start() error {
	return c.wrapError(c.Cmd.Start())
}

func (c *Cmd) Wait() error {
	return c.wrapError(c.Cmd.Wait())
}

func (c *Cmd) StdinPipe() (io.WriteCloser, error) {
	c.Cmd.Stdin = nil
	wc, err := c.Cmd.StdinPipe()
	return wc, c.wrapError(err)
}

func (c *Cmd) StdoutPipe() (io.ReadCloser, error) {
	c.Cmd.Stdout = nil
	rc, err := c.Cmd.StdoutPipe()
	return rc, c.wrapError(err)
}

func (c *Cmd) StderrPipe() (io.ReadCloser, error) {
	c.Cmd.Stderr = nil
	rc, err := c.Cmd.StderrPipe()
	return rc, c.wrapError(err)
}

func (c *Cmd) Output() ([]byte, error) {
	c.Cmd.Stdout = nil
	out, err := c.Cmd.Output()
	return out, c.wrapError(err)
}

func (c *Cmd) OutputString() (string, error) {
	if out, err := c.Output(); err != nil {
		return "", c.wrapError(err)
	} else {
		return strings.TrimSuffix(string(out), "\n"), nil
	}
}

func (c *Cmd) OutputLines() ([]string, error) {
	if out, err := c.OutputString(); err != nil {
		return nil, err
	} else {
		return strings.Split(out, "\n"), nil
	}
}

func (c *Cmd) ReadFrom(r io.Reader) *Cmd {
	c.Cmd.Stdin = r
	return c
}

func (c *Cmd) WriteTo(w io.Writer) *Cmd {
	c.Cmd.Stdout = w
	return c
}
