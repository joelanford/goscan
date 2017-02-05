package words

import (
	"io/ioutil"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type DirtyWord struct {
	Word      string   `yaml:"word"`
	Blacklist []string `yaml:"blacklist"`
	Reason    string   `yaml:"reason"`
}

type Hit struct {
	Word      string
	File      string
	FileIndex int
	Context   string
}

func LoadFile(wordsFile string) ([]DirtyWord, error) {
	data, err := ioutil.ReadFile(wordsFile)
	if err != nil {
		return nil, errors.Wrap(err, "error reading words file")
	}
	var dirtyWords []DirtyWord
	err = yaml.Unmarshal(data, &dirtyWords)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing words file yaml")
	}
	return dirtyWords, nil
}
