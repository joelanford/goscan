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
	scratchDir string
	baseDir    string
}

func New(baseDir string) *Scratch {
	return &Scratch{
		baseDir: baseDir,
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
	err := os.RemoveAll(s.scratchDir)
	if err != nil {
		return errors.Wrap(err, "error deleting temporary directory")
	}
	s.scratchDir = ""
	return nil
}
