package run

import "fmt"
import "io"
import "os"
import "os/exec"
import "strings"

import "lib/ui"

type Cmd struct {
	Cmd exec.Cmd
}

func (c *Cmd) commandString() string {
	return ShellEscape(c.Cmd.Args...)
}

func (c *Cmd) String() string {
	return fmt.Sprintf("run.Command[%s]", c.commandString())
}

type CmdError struct {
	ExecError error
	Cmd       *Cmd
}

func (err *CmdError) Error() string {
	return fmt.Sprintf("%v: %v", err.Cmd, err.ExecError)
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
	if ui.Debug {
		fmt.Fprintf(os.Stderr, "+ %v\n", c.commandString())
	}
	return c.wrapError(c.Cmd.Run())
}

func (c *Cmd) Start() error {
	if ui.Debug {
		defer func() {
			pid := -1
			if p := c.Cmd.Process; p != nil {
				pid = p.Pid
			}
			fmt.Fprintf(os.Stderr, "+ %v & [%v]\n", c.commandString(), pid)
		}()
	}
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
	if ui.Debug {
		fmt.Fprintf(os.Stderr, "+ %v |", c.commandString())
	}
	c.Cmd.Stdout = nil
	out, err := c.Cmd.Output()
	if ui.Debug {
		fmt.Fprintf(os.Stderr, " %#v\n", string(out))
	}
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
		if out == "" {
			return nil, nil
		}
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
