//go:build darwin

package atomicfile

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"

	"golang.org/x/sys/unix"
)

type Option interface {
	apply(*config) error
}

type optionFunc func(*config) error

func (o optionFunc) apply(cfg *config) error {
	return o(cfg)
}

func Contents(r io.Reader) Option {
	return optionFunc(func(c *config) error {
		if c.contents != nil {
			return fmt.Errorf("multiple contents")
		}
		c.contents = r
		return nil
	})
}

func Fsync() Option {
	return optionFunc(func(c *config) error {
		c.flushData = true
		return nil
	})
}

func Preallocate(size int64) Option {
	return optionFunc(func(c *config) error {
		if c.prealloc > 0 {
			return fmt.Errorf("multiple preallocations")
		}
		if size < 0 {
			return fmt.Errorf("invalid preallocation size: %d", size)
		}
		c.prealloc = size
		return nil
	})
}

// TODO: owner/group, permissions, file times, lock, xattr, fadvise flags, fsync, ...

type config struct {
	contents  io.Reader
	flushData bool
	prealloc  int64
}

func Create(filename string, options ...Option) error {
	var cfg config
	for _, o := range options {
		if err := o.apply(&cfg); err != nil {
			return &werror{"options", err}
		}
	}

	dir := path.Dir(filename)

	var d *os.File
	if cfg.flushData {
		// on Linux the directory fd can be opened as read-only for fsync
		d, err := os.OpenFile(dir, unix.O_DIRECTORY|os.O_RDONLY, 0)
		if err != nil {
			return &werror{"opening directory", err}
		}
		defer d.Close()
	}

	f, err := os.OpenFile(dir, _O_TMPFILE|os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o777)
	if err != nil {
		return &werror{"opening file", err}
	}
	defer f.Close()

	prealloc := cfg.prealloc
	if prealloc == 0 && cfg.contents != nil {
		if guess := guessContentSize(cfg.contents); guess > 0 {
			prealloc = guess
		}
	}
	if prealloc > 0 {
		err := unix.Fallocate(f.Fd(), unix.FALLOC_FL_KEEP_SIZE, 0, prealloc)
		if cfg.prealloc > 0 && err != nil {
			return &werror{"preallocating file", err}
		}
	}

	if cfg.contents != nil {
		_, err := io.Copy(f, cfg.contents)
		if err != nil {
			return &werror{"populating file", err}
		}
	}

	if cfg.flushData {
		err := unix.Fsync(int(f.Fd()))
		if err != nil {
			return &werror{"fsync file", err}
		}
	}

	err = unix.Linkat(int(f.Fd()), "", unix.AT_FDCWD, filename, _AT_EMPTY_PATH)
	if err != nil {
		procPath := "/proc/self/fd/" + strconv.Itoa(int(f.Fd()))
		err2 := unix.Linkat(unix.AT_FDCWD, procPath, unix.AT_FDCWD, filename, unix.AT_SYMLINK_FOLLOW)
		if err2 != nil {
			return &werror{"linking file", err}
		}
	}

	if cfg.flushData {
		err := unix.Fsync(int(d.Fd()))
		if err != nil {
			return &werror{"fsync directory", err}
		}
	}

	return nil
}

const (
	_O_TMPFILE     = unix.O_DIRECTORY | 0o20000000
	_AT_EMPTY_PATH = 0x1000
)

type werror struct {
	msg   string
	cause error
}

func (e *werror) Error() string {
	if e.cause == nil {
		return e.msg
	}
	return e.msg + ": " + e.cause.Error()
}

func (e *werror) Unwrap() error {
	return e.cause
}

func guessContentSize(r io.Reader) int64 {
	switch r := r.(type) {
	case *bytes.Buffer:
		return int64(r.Len())
	case *os.File:
		fi, err := r.Stat()
		if err != nil || !fi.Mode().IsRegular() {
			return 0
		}
		return fi.Size()
	}
	return 0
}
