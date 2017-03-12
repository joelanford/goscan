package scratch

import (
	"io"
	"os"
	"path"
	"strings"

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

func (s *Scratch) CopyFile(ifilename string) (string, error) {
	ifiledir := path.Dir(ifilename)
	var ofiledir string
	if path.IsAbs(ifiledir) {
		ofiledir = path.Clean(path.Join(s.Dir(), strings.Replace(ifiledir, ":", "_", -1)))
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		ofiledir = path.Clean(path.Join(s.Dir(), strings.Replace(cwd, ":", "_", -1), ifiledir))
	}
	ofilename := path.Join(ofiledir, path.Base(ifilename))

	ifile, err := os.Open(ifilename)
	if err != nil {
		return "", err
	}
	defer ifile.Close()
	err = os.MkdirAll(path.Dir(ofilename), 0777)
	if err != nil {
		return "", err
	}
	ofile, err := os.Create(ofilename)
	if err != nil {
		return "", err
	}
	defer ofile.Close()
	if _, err := io.Copy(ofile, ifile); err != nil {
		return "", err
	}
	return ofilename, nil
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
