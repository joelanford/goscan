package scratch

import (
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

func (s *Scratch) attach() error {
	return nil
}

func (s *Scratch) mount() error {
	var output []byte
	var err error

	err = os.Mkdir(s.path, 0777)
	if err != nil {
		return errors.Wrapf(err, "error creating temporary directory for mount point")
	}

	output, err = exec.Command("mount", "-t", "tmpfs", "-o", "noatime,size="+s.ramdiskMegabytes+"m", r.mountPoint).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "error creating temporary directory for mount point:"+string(output))
	}

	return nil
}

func (s *Scratch) detach() error {
	output, err := exec.Command("umount", "-l", s.path).CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	return nil
}
