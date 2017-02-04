package ramdisk

import (
	"fmt"
	"os/exec"

	"io/ioutil"

	"os"

	"strings"

	"github.com/pkg/errors"
)

type Ramdisk struct {
	name       string
	mountPoint string
	megabytes  int

	device string
}

func New(name string, megabytes int) *Ramdisk {
	return &Ramdisk{
		name:      name,
		megabytes: megabytes,
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

	r.mountPoint, err = ioutil.TempDir("/tmp", "goscan-")
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
