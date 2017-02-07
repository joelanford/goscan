package scratch

import (
	"os"

	"github.com/pkg/errors"
)

type Opts struct {
	Path             string
	RamdiskEnable    bool
	RamdiskMegabytes int
}

type Scratch struct {
	path             string
	ramdiskMegabytes int
	ramdiskEnable    bool

	device string
}

func New(opts Opts) *Scratch {
	return &Scratch{
		path:             opts.Path,
		ramdiskEnable:    opts.RamdiskEnable,
		ramdiskMegabytes: opts.RamdiskMegabytes,
	}
}

func (s *Scratch) Setup() error {
	if s.ramdiskEnable {
		err := s.attach()
		if err != nil {
			return errors.Wrap(err, "error attaching ramdisk")
		}
		err = s.mount()
		if err != nil {
			return errors.Wrap(err, "error mounting ramdisk")
		}
	} else {
		if _, err := os.Stat(s.path); os.IsNotExist(err) {
			err = os.Mkdir(s.path, 0777)
			if err != nil {
				return errors.Wrap(err, "error creating temporary directory")
			}
		}
	}
	return nil
}

func (s *Scratch) Teardown() error {
	if s.ramdiskEnable {
		err := s.detach()
		if err != nil {
			return errors.Wrap(err, "error unmounting ramdisk")
		}
	}
	err := os.RemoveAll(s.path)
	if err != nil {
		return errors.Wrap(err, "error deleting temporary directory")
	}
	return nil
}
