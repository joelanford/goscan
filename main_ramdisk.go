// +build linux darwin

package main

import (
	"flag"

	"github.com/joelanford/goscan/utils/scratch"
)

func parseScratchOpts(opts *scratch.Opts) {
	flag.StringVar(&opts.BasePath, "scratch.basedir", "", "Scratch directory for scan unarchiving")
	flag.BoolVar(&opts.RamdiskEnable, "scratch.ramdisk.enable", false, "Enable ramdisk scratch directory")
	flag.IntVar(&opts.RamdiskMegabytes, "scratch.ramdisk.mb", 4096, "Size of ramdisk to use as scratch space")
}
