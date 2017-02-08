package scratch

import (
	"os"

	"github.com/pkg/errors"
	"io/ioutil"
)

type Opts struct {
	BasePath         string
	RamdiskEnable    bool
	RamdiskMegabytes int
}

type Scratch struct {
	ScratchSpacePath string
	basePath         string
	ramdiskMegabytes int
	ramdiskEnable    bool

	device string
}

func New(opts Opts) *Scratch {
	return &Scratch{
		basePath:         opts.BasePath,
		ramdiskEnable:    opts.RamdiskEnable,
		ramdiskMegabytes: opts.RamdiskMegabytes,
	}
}

func (s *Scratch) Setup() error {
	var err error
	s.ScratchSpacePath, err = ioutil.TempDir(s.basePath, "goscan")
	if err != nil {
		return err
	}
	if s.ramdiskEnable {
		err := s.attach()
		if err != nil {
			return errors.Wrap(err, "error attaching ramdisk")
		}
		err = s.mount()
		if err != nil {
			return errors.Wrap(err, "error mounting ramdisk")
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
	err := os.RemoveAll(s.ScratchSpacePath)
	if err != nil {
		return errors.Wrap(err, "error deleting temporary directory")
	}
	return nil
}
