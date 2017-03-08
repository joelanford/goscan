// +build linux darwin

package app

import "flag"

func configureRamdiskOpts(opts *ScanOpts) {
	flag.BoolVar(&opts.RamdiskEnable, "ramdisk.enable", false, "Enable ramdisk scratch directory")
	flag.IntVar(&opts.RamdiskSize, "ramdisk.size", 4096, "Size of ramdisk (in MB) to use as scratch space")
}
