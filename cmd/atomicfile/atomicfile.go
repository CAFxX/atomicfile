package main

import (
	"os"
	"strconv"
	"time"

	"github.com/CAFxX/atomicfile"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	filename := kingpin.Arg("filename", "Name of the file to create").Required().String()
	fsync := kingpin.Flag("fsync", "Fsync the file").Default("false").Bool()
	prealloc := kingpin.Flag("prealloc", "Preallocate file space (bytes)").Default("0").Int64()
	xattrs := kingpin.Flag("xattr", "Extended attributes to be added to the file").PlaceHolder("KEY=VALUE").StringMap()
	perm := kingpin.Flag("perm", "File permissions").String()
	uid := kingpin.Flag("uid", "File owner user").Default("-1").PlaceHolder("UID").Int()
	gid := kingpin.Flag("gid", "File owner group").Default("-1").PlaceHolder("GID").Int()
	mtime := kingpin.Flag("mtime", "File modification time (RFC 3339)").String()
	atime := kingpin.Flag("atime", "File access time (RFC 3339)").String()
	kingpin.Parse()

	opts := []atomicfile.Option{
		atomicfile.Contents(os.Stdin),
	}
	if *fsync {
		opts = append(opts, atomicfile.Fsync())
	}
	if *prealloc != 0 {
		opts = append(opts, atomicfile.Preallocate(*prealloc))
	}
	for k, v := range *xattrs {
		opts = append(opts, atomicfile.Xattr(k, []byte(v)))
	}
	if *perm != "" {
		pp, err := strconv.ParseUint(*perm, 8, 32)
		if err != nil {
			fatal(err)
		}
		opts = append(opts, atomicfile.Permissions(os.FileMode(pp)))
	}
	if *uid != -1 || *gid != -1 {
		opts = append(opts, atomicfile.Ownership(*uid, *gid))
	}
	if *mtime != "" {
		t, err := time.Parse(time.RFC3339Nano, *mtime)
		if err != nil {
			fatal(err)
		}
		opts = append(opts, atomicfile.ModificationTime(t))
	}
	if *atime != "" {
		t, err := time.Parse(time.RFC3339Nano, *atime)
		if err != nil {
			fatal(err)
		}
		opts = append(opts, atomicfile.AccessTime(t))
	}

	err := atomicfile.Create(*filename, opts...)
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	os.Stderr.WriteString(err.Error())
	os.Stderr.WriteString("\n")
	os.Exit(-1)
}
