// Package logfile is the program's log file, which does not grow without limit: when it passes a size
// it is renamed to name.1 (replacing the one that was there) and a new, empty file is started, so the
// log never takes more than about twice the limit.
package logfile

import (
	"os"
	"sync"
)

// DefaultMaxBytes is the size at which the log is rotated.
const DefaultMaxBytes = 5 << 20

// Writer writes to a file and rotates it. It is safe for use from several goroutines.
type Writer struct {
	mu   sync.Mutex
	path string
	max  int64
	f    *os.File
	size int64
}

// Open opens (or creates) the log at path, appending to it, and rotates it at max bytes (0: the default).
// A file that is already over the limit is rotated at once.
func Open(path string, max int64) (*Writer, error) {
	if max <= 0 {
		max = DefaultMaxBytes
	}
	w := &Writer{path: path, max: max}
	if err := w.open(); err != nil {
		return nil, err
	}
	if w.size >= w.max {
		if err := w.rotate(); err != nil {
			w.f.Close()
			return nil, err
		}
	}
	return w, nil
}

func (w *Writer) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.f, w.size = f, st.Size()
	return nil
}

// rotate moves the log to path.1 and starts a new one. When the file cannot be moved (another program
// holds it open, which Windows does not allow) it is emptied instead, so it still stays small.
func (w *Writer) rotate() error {
	w.f.Close()
	if err := os.Remove(w.path + ".1"); err != nil && !os.IsNotExist(err) {
		return w.truncate()
	}
	if err := os.Rename(w.path, w.path+".1"); err != nil {
		return w.truncate()
	}
	return w.open()
}

func (w *Writer) truncate() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	w.f, w.size = f, 0
	return nil
}

// Write implements io.Writer. A failure to rotate does not lose the line: it goes to the file that is open.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size+int64(len(p)) > w.max && w.size > 0 {
		_ = w.rotate()
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

// Close closes the file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}
