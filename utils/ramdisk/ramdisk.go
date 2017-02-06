package ramdisk

import (
	"os"

	"github.com/pkg/errors"
)

type Opts struct {
	Name      string
	BaseDir   string
	Megabytes int
}

type Ramdisk struct {
	name      string
	basedir   string
	megabytes int

	mountPoint string
	device     string
}

func New(opts Opts) *Ramdisk {
	return &Ramdisk{
		name:      opts.Name,
		basedir:   opts.BaseDir,
		megabytes: opts.Megabytes,
	}
}

func (r *Ramdisk) MountPoint() string {
	return r.mountPoint
}

func (r *Ramdisk) Device() string {
	return r.device
}

func (r *Ramdisk) Mount() error {
	err := r.attach()
	if err != nil {
		return errors.Wrap(err, "error mounting ramdisk")
	}
	err = r.mount()
	return nil
}

func (r *Ramdisk) Unmount() error {
	err := r.detach()
	if err != nil {
		return errors.Wrap(err, "error unmounting ramdisk")
	}
	err = os.RemoveAll(r.mountPoint)
	if err != nil {
		return errors.Wrap(err, "error deleting temporary mountpoint")
	}
	r.mountPoint = ""
	return nil
}
