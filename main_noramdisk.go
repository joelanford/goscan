// +build !linux,!darwin

package main

import (
	"flag"

	"github.com/joelanford/goscan/utils/scratch"
)

func parseScratchOpts(opts *scratch.Opts) {
	flag.StringVar(&opts.Path, "scratch.dir", "", "Scratch directory for scan unarchiving")
	opts.RamdiskEnable = false
}
