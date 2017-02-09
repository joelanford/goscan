// +build !linux,!darwin

package main

import (
	"flag"

	"github.com/joelanford/goscan/utils/scratch"
)

func configureScratchOpts(opts *scratch.Opts) {
	flag.StringVar(&opts.BasePath, "scratch.basedir", "", "Scratch directory for scan unarchiving")
	opts.RamdiskEnable = false
}
