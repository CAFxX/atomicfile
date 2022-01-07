package main

import (
	"flag"
	"os"

	"github.com/CAFxX/atomicfile"
)

func main() {
	fsync := flag.Bool("fsync", false, "Fsync the file")
	flag.Parse()
	filename := flag.Arg(0)

	opts := []atomicfile.Option{
		atomicfile.Contents(os.Stdin),
	}
	if *fsync {
		opts = append(opts, atomicfile.Fsync())
	}

	err := atomicfile.Create(filename, opts...)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(-1)
	}
}
