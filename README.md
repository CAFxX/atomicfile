# atomicfile

Linux command to atomically create fully-formed files with contents read from stdin.

```sh
# Atomically create a file called hello.txt with the contents passed on stdin
echo 'Hello world!' | atomicfile hello.txt
```

Files are always created atomically using `O_TMPFILE`/`linkat`, so any other process
in the system can not observe the file in an incomplete state (this includes all
aspects of the file specified as flags to the command, such as xattrs, permissions,
owner, etc.).

If durability is required in addition to atomicity the `--fsync` flag will call
`fsync(2)` on both the file being created and on the directory containing it.

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
