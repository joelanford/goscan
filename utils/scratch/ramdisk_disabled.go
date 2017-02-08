// +build !linux,!darwin

package scratch

func (s *Scratch) attach() error {
	panic("ramdisk not supported")
}

func (s *Scratch) mount() error {
	panic("ramdisk not supported")
}

func (s *Scratch) detach() error {
	panic("ramdisk not supported")
}
