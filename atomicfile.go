//go:build linux
// +build linux

package atomicfile

import (
	"bytes"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Option is the interface for options passed to Create.
type Option interface {
	apply(*config) error
}

type optionFunc func(*config) error

func (o optionFunc) apply(cfg *config) error {
	return o(cfg)
}

// Contents specifies the contents to be written to the target file.
func Contents(r io.Reader) Option {
	return optionFunc(func(c *config) error {
		if c.contents != defaultConfig().contents {
			return &werror{"multiple contents", nil}
		}
		c.contents = r
		return nil
	})
}

// Fsync enables the invocation of fsync() on the target file and
// its containing directory.
func Fsync() Option {
	return optionFunc(func(c *config) error {
		c.flushData = true
		return nil
	})
}

// Preallocate allocates the specified amount of bytes in the target
// file, regardless of the amount of content written.
// Not all filesystems and kernel versions support preallocating space.
func Preallocate(size int64) Option {
	return optionFunc(func(c *config) error {
		if c.prealloc != defaultConfig().prealloc {
			return &werror{"multiple preallocations", nil}
		}
		if size < 0 {
			return &werror{"invalid preallocation size", nil}
		}
		c.prealloc = size
		return nil
	})
}

// Xattr specifies an extended attribute to be added to the target file.
// Multiple externded attributes can be added to the same file.
// Not all filesystems and kernel versions support extended attributes.
func Xattr(name string, value []byte) Option {
	return optionFunc(func(c *config) error {
		c.xattrs = append(c.xattrs, struct {
			name  string
			value []byte
		}{name, value})
		return nil
	})
}

// Permissions specifies the Unix permissions to be set on the target file.
func Permissions(mode os.FileMode) Option {
	return optionFunc(func(c *config) error {
		if c.perm != defaultConfig().perm {
			return &werror{"multiple permissions", nil}
		}
		c.perm = uint32(mode.Perm())
		return nil
	})
}

// Ownership specifies the target file owner UID and GID.
func Ownership(uid, gid int) Option {
	return optionFunc(func(c *config) error {
		if c.uid != defaultConfig().uid || c.gid != defaultConfig().gid {
			return &werror{"multiple ownership", nil}
		}
		c.uid, c.gid = uid, gid
		return nil
	})
}

// ModificationTime specifies the modification time of the target file.
func ModificationTime(t time.Time) Option {
	return optionFunc(func(c *config) error {
		if c.mtime != defaultConfig().mtime {
			return &werror{"multiple modification times", nil}
		}
		ts, err := unix.TimeToTimespec(t)
		if err != nil {
			return &werror{"invalid modification time", err}
		}
		c.mtime = ts
		return nil
	})
}

// AccessTime specifies the access time of the target file.
func AccessTime(t time.Time) Option {
	return optionFunc(func(c *config) error {
		if c.atime != defaultConfig().atime {
			return &werror{"multiple access times", nil}
		}
		ts, err := unix.TimeToTimespec(t)
		if err != nil {
			return &werror{"invalid access time", err}
		}
		c.atime = ts
		return nil
	})
}

// TODO: owner/group, permissions, file times, lock, xattr, fadvise flags, fsync, ...

type config struct {
	contents  io.Reader
	flushData bool
	prealloc  int64
	xattrs    []struct {
		name  string
		value []byte
	}
	perm  uint32
	uid   int
	gid   int
	mtime unix.Timespec
	atime unix.Timespec
}

func defaultConfig() config {
	return config{
		perm:  ^uint32(0),
		uid:   -1,
		gid:   -1,
		mtime: unix.Timespec{Nsec: unix.UTIME_OMIT},
		atime: unix.Timespec{Nsec: unix.UTIME_OMIT},
	}
}

// Create creates the specified file with the provided options.
// The file is created atomically in a fully-formed state using
// O_TMPFILE/linkat.
// Create fails if the file already exists.
func Create(filename string, options ...Option) error {
	cfg := defaultConfig()
	for _, o := range options {
		if err := o.apply(&cfg); err != nil {
			return &werror{"options", err}
		}
	}

	dir := path.Dir(filename)

	var d *os.File
	var err error
	if cfg.flushData {
		// on Linux the directory fd can be opened as read-only for fsync
		d, err = os.OpenFile(dir, unix.O_DIRECTORY|os.O_RDONLY, 0)
		if err != nil {
			return &werror{"opening directory", err}
		}
		// TODO: check error
		defer d.Close()
	}

	f, err := os.OpenFile(dir, unix.O_TMPFILE|os.O_APPEND|os.O_WRONLY, 0o666)
	if err != nil {
		return &werror{"opening file", err}
	}
	// TODO: check error
	defer f.Close()

	if cfg.uid != defaultConfig().uid || cfg.gid != defaultConfig().gid {
		err := unix.Fchown(int(f.Fd()), cfg.uid, cfg.gid)
		if err != nil {
			return &werror{"setting ownership", err}
		}
	}

	if cfg.perm != defaultConfig().perm {
		err := unix.Fchmod(int(f.Fd()), cfg.perm)
		if err != nil {
			return &werror{"setting permissions", err}
		}
	}

	prealloc := cfg.prealloc
	if prealloc == defaultConfig().prealloc && cfg.contents != nil {
		if guess := guessContentSize(cfg.contents); guess > 0 {
			prealloc = guess
		}
	}
	if prealloc > 0 {
		err := unix.Fallocate(int(f.Fd()), unix.FALLOC_FL_KEEP_SIZE, 0, prealloc)
		if err != nil {
			prealloc = 0
			if cfg.prealloc > 0 {
				return &werror{"preallocating file", err}
			}
		}
	}

	var written int64
	if cfg.contents != nil {
		written, err = io.Copy(f, cfg.contents)
		if err != nil {
			return &werror{"populating file", err}
		}
	}

	if written < prealloc && cfg.prealloc > 0 {
		// TODO: should we fail in this case?
		_ = unix.Fallocate(int(f.Fd()), unix.FALLOC_FL_PUNCH_HOLE|unix.FALLOC_FL_KEEP_SIZE, written, prealloc-written)
	}

	for _, xattr := range cfg.xattrs {
		err := unix.Fsetxattr(int(f.Fd()), xattr.name, xattr.value, 0)
		if err != nil {
			return &werror{"setting xattr", err}
		}
	}

	if cfg.mtime != defaultConfig().mtime || cfg.atime != defaultConfig().atime {
		err := futimens(int(f.Fd()), &[2]unix.Timespec{cfg.atime, cfg.mtime})
		if err != nil {
			return &werror{"setting access/modification time", err}
		}
	}

	if cfg.flushData {
		err := f.Sync()
		if err != nil {
			return &werror{"fsync file", err}
		}
	}

	const AT_EMPTY_PATH = 0x1000
	err = unix.Linkat(int(f.Fd()), "", unix.AT_FDCWD, filename, AT_EMPTY_PATH)
	if err != nil {
		procPath := "/proc/self/fd/" + strconv.Itoa(int(f.Fd()))
		err2 := unix.Linkat(unix.AT_FDCWD, procPath, unix.AT_FDCWD, filename, unix.AT_SYMLINK_FOLLOW)
		if err2 != nil {
			return &werror{"linking file", err2}
		}
	}

	if cfg.flushData {
		err := d.Sync()
		if err != nil {
			return &werror{"fsync directory", err}
		}
	}

	return nil
}

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
	case *strings.Reader:
		return int64(r.Len())
	case *os.File:
		fi, err := r.Stat()
		if err != nil || !fi.Mode().IsRegular() {
			return 0
		}
		return fi.Size()
	case *io.SectionReader:
		pos, err := r.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0
		}
		return r.Size() - pos
	case *io.LimitedReader:
		n := guessContentSize(r.R)
		if n == 0 || n < r.N {
			return n
		}
		return r.N
	}
	return 0
}

// https://github.com/golang/go/issues/49699
func futimens(fd int, times *[2]unix.Timespec) (err error) {
	_, _, e1 := unix.Syscall6(unix.SYS_UTIMENSAT, uintptr(fd), 0, uintptr(unsafe.Pointer(times)), 0, 0, 0)
	if e1 != 0 {
		err = e1
	}
	return
}
