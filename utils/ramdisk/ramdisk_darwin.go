package ramdisk

import (
	"fmt"
	"os/exec"

	"io/ioutil"

	"strings"

	"github.com/pkg/errors"
)

func (r *Ramdisk) attach() error {
	output, err := exec.Command("hdiutil", "attach", "-nomount", fmt.Sprintf("ram://%d", r.megabytes*2048)).CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	r.device = strings.TrimSpace(string(output))
	return nil
}

func (r *Ramdisk) mount() error {
	var output []byte
	var err error

	r.mountPoint, err = ioutil.TempDir(r.basedir, "goscan-")
	if err != nil {
		return errors.Wrapf(err, "error creating temporary directory for mount point")
	}

	output, err = exec.Command("newfs_hfs", "-v", r.name, r.device).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "error creating hfs filesystem for ramdisk: "+string(output))
	}

	output, err = exec.Command("mount", "-o", "noatime", "-t", "hfs", r.device, r.mountPoint).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "error creating temporary directory for mount point:"+string(output))
	}

	return nil
}

func (r *Ramdisk) detach() error {
	output, err := exec.Command("hdiutil", "detach", r.device, "-force").CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	r.device = ""
	return nil
}
