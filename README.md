# atomicfile

Linux command to atomically create fully-formed files with contents read from stdin.

## Examples

```sh
# Atomically create a file called hello.txt with the contents passed on stdin.
echo 'Hello world!' | atomicfile hello.txt

# Atomically create a copy of the source file contents (also across filesystems),
# minimizing block cache pollution and requesting durability for the new file.
# Note that the source file is not read atomically.
atomicfile --dontneed --fsync destination.file </some/other/filesystem/source.file

# Atomically create an empty file called foo preallocated with 100000 bytes,
# custom permissions, and an extended attribute.
atomicfile --perm 600 --prealloc 100000 --xattr user.mykey=myValue foo
```

Files are always created atomically using `O_TMPFILE`/`linkat`, so any other process
in the system can not observe the file in an incomplete state (this includes all
aspects of the file specified as flags to the command, such as xattrs, permissions,
owner, etc.). See the [usage](#usage) section for details about all supported controls.

In addition, the same (and additional) functionalities are exposed as a
[Go library](https://pkg.go.dev/github.com/CAFxX/atomicfile):

```golang
// import github.com/CAFxX/atomicfile

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
  --dontneed             Minimize block cache usage
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
