package scratch

import (
	"os"

	"io/ioutil"

	"github.com/pkg/errors"
)

type Scratch struct {
	scratchDir    string
	baseDir       string
	ramdiskSize   int
	ramdiskEnable bool

	device string
}

func New(baseDir string, enable bool, size int) *Scratch {
	return &Scratch{
		baseDir:       baseDir,
		ramdiskEnable: enable,
		ramdiskSize:   size,
	}
}

func (s *Scratch) Dir() string {
	return s.scratchDir
}

func (s *Scratch) Setup() error {
	var err error
	s.scratchDir, err = ioutil.TempDir(s.baseDir, "goscan")
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
	err := os.RemoveAll(s.scratchDir)
	if err != nil {
		return errors.Wrap(err, "error deleting temporary directory")
	}
	s.scratchDir = ""
	return nil
}
