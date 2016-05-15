package wraperr

import "fmt"

type WrappedErr struct {
	Err  error
	Note string
}

func (we *WrappedErr) Error() string {
	if we.Note == "" {
		return we.Err.Error()
	}
	return fmt.Sprintf("%s (%s)", we.Note, we.Err.Error())
}

func (we *WrappedErr) String() string {
	return we.Error()
}

func Wrapf(err error, format string, args ...interface{}) error {
	if _, alreadyWrapped := err.(*WrappedErr); alreadyWrapped {
		// Discard note
		return err
	}
	// TODO: format backtrace & stuff
	return &WrappedErr{
		Err:  err,
		Note: fmt.Sprintf(format, args...),
	}
}
