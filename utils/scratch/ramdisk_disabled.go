// +build !linux,!darwin

package scratch

func (r *Ramdisk) attach() error {
	panic("ramdisk not supported")
}

func (r *Ramdisk) mount() error {
	panic("ramdisk not supported")
}

func (r *Ramdisk) detach() error {
	panic("ramdisk not supported")
}
