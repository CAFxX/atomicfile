# atomicfile

Linux command to atomically create fully-formed files with contents read from stdin.

```sh
# Atomically create a file called hello.txt with the contents passed on stdin
echo 'Hello world!' | atomicfile hello.txt

# Atomically copy a file (also across filesystems)
atomicfile destination.file </some/other/filysystem/source.file
```

Files are always created atomically using `O_TMPFILE`/`linkat`, so any other process
in the system can not observe the file in an incomplete state (this includes all
aspects of the file specified as flags to the command, such as xattrs, permissions,
owner, etc.).

In addition, the same functionality is exposed as a [Go library](https://pkg.go.dev/github.com/CAFxX/atomicfile):

```golang
// atomically create a file with contents read from the provided io.Reader r
err := atomicfile.Create(filename, atomicfile.Contents(r))
if err != nil {
  panic(err)
}
```

## Install

```
go install github.com/CAFxX/atomicfile/cmd/atomicfile@latest
```

## Usage

```
usage: atomicfile [<flags>] <filename>

Flags:
  --help                 Show context-sensitive help (also try --help-long and --help-man).
  --fsync                Fsync the file
  --prealloc=0           Preallocate file space (bytes)
  --xattr=KEY=VALUE ...  Extended attributes to be added to the file
  --perm=PERM            File permissions
  --uid=UID              File owner user
  --gid=GID              File owner group
  --mtime=MTIME          File modification time (RFC 3339)
  --atime=ATIME          File access time (RFC 3339)

Args:
  <filename>  Name of the file to create
```

### Requirements

- `atomicfile` requires Linux >= 3.11 (for `O_TMPFILE`).
- Availability of some of the features (preallocating space, extended attributes, ...)
  depend on the filesystem and kernel version.
- Setting UID/GID normally requires the process to run with elevated privileges (sudo).
