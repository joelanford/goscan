// +build !linux,!darwin

package app

func configureRamdiskOpts(opts *ScanOpts) {
	opts.RamdiskEnable = false
	opts.RamdiskSize = 0
}
