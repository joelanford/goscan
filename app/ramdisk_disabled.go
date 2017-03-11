// +build !linux,!darwin

package app

func configureRamdiskOpts(opts *Opts) {
	opts.RamdiskEnable = false
	opts.RamdiskSize = 0
}
