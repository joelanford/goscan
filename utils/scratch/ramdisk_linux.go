package scratch

import (
	"os/exec"
	"strconv"

	"github.com/pkg/errors"
)

func (s *Scratch) attach() error {
	return nil
}

func (s *Scratch) mount() error {
	var output []byte
	var err error

	mb := strconv.Itoa(s.ramdiskMegabytes)
	output, err = exec.Command("mount", "-t", "tmpfs", "-o", "noatime,size="+mb+"m", s.ScratchSpacePath).CombinedOutput()

	if err != nil {
		return errors.Wrapf(err, "error creating temporary directory for mount point:"+string(output))
	}

	return nil
}

func (s *Scratch) detach() error {
	output, err := exec.Command("umount", "-l", s.ScratchSpacePath).CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	return nil
}
