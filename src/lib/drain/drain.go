package drain

import (
	"bytes"
	"errors"
	"time"
)

var ErrClosed = errors.New("Closed")

type Line struct {
	Writer    *Writer
	Text      string
	Timestamp time.Time
}

type Drain chan Line

type Writer struct {
	drain   Drain
	pending []byte
	ts      time.Time
}

func (drain Drain) NewWriter() *Writer {
	return &Writer{drain: drain}
}

func (drain Drain) Lines() []Line {
	rv := []Line{}
	for line := range drain {
		rv = append(rv, line)
	}
	return rv
}

func (w *Writer) emit(line []byte, timestamp time.Time) {
	// fmt.Fprintf(os.Stderr, " EMIT: %v %#v\n", timestamp, string(line))
	w.drain <- Line{w, string(line), timestamp}
}

func (w *Writer) Write(p_ []byte) (n int, err error) {
	if w.drain == nil {
		return 0, ErrClosed
	}
	ts := time.Now()

	// Writer is not allowed to modify or retain the original slice. We
	// may optimize it later on, but for now we just make ourselves a
	// copy.
	p := make([]byte, len(p_))
	copy(p, p_)

	// fmt.Fprintf(os.Stderr, "WRITE: %v %d:[%#v]%#v\n", ts, len(p), string(w.pending), string(p))

	pending := len(w.pending) > 0
	if pending {
		p = append(w.pending, p...)
	}

	if lines := bytes.Split(p, []byte("\n")); len(lines) == 1 {
		// no newline => save as pending
		if !pending {
			// no previously pending partial line, set timestamp
			w.ts = ts
		}
		// If we are continuing a previously pending line, we leave the timestamp unchanged
		w.pending = p
		// fmt.Fprintf(os.Stderr, "-PEND: %v %#v\n", w.ts, string(w.pending))
	} else {
		// at least 2 lines
		if pending {
			// There was a pending line, emit first line with its timestamp
			w.emit(lines[0], w.ts)
			lines = lines[1:]
		}
		// at least 1 line now
		for _, ln := range lines[:len(lines)-1] {
			w.emit(ln, ts)
		}
		w.pending = lines[len(lines)-1]
		w.ts = ts
		// fmt.Fprintf(os.Stderr, " PEND: %v %#v\n", w.ts, string(w.pending))
	}
	return len(p_), nil
}

func (w *Writer) Flush() error {
	if w.drain == nil {
		return ErrClosed
	}
	if len(w.pending) > 0 {
		w.emit(w.pending, w.ts)
		w.pending = nil
	}
	return nil
}

func (w *Writer) Close() error {
	w.Flush()
	w.drain = nil
	return nil
}
