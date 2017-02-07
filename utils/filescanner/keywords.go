package filescanner

import (
	"io/ioutil"

	"github.com/joelanford/goscan/utils/ahocorasick"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type Keywords struct {
	Keywords []Keyword `yaml:"keywords"`

	dictionary *ahocorasick.Machine
}

type Keyword struct {
	Word      string   `yaml:"word"`
	Blacklist []string `yaml:"blacklist"`
	Reason    string   `yaml:"reason"`
}

func LoadKeywords(wordsFile string) (*Keywords, error) {
	data, err := ioutil.ReadFile(wordsFile)
	if err != nil {
		return nil, errors.Wrap(err, "error reading keywords file")
	}

	var keywords Keywords
	err = yaml.Unmarshal(data, &keywords)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing keywords file")
	}

	var keywordsBytes [][]byte
	for _, keyword := range keywords.Keywords {
		keywordsBytes = append(keywordsBytes, []byte(keyword.Word))
	}

	keywords.dictionary = &ahocorasick.Machine{}
	keywords.dictionary.Build(keywordsBytes)

	return &keywords, nil
}

func (k *Keywords) MatchFile(file string) ([]Hit, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	hits := make([]Hit, 0)
	for _, t := range k.dictionary.MultiPatternSearch(data, false) {
		ctxBegin := t.Pos - 20
		ctxEnd := t.Pos + len(t.Word) + 20
		if ctxBegin < 0 {
			ctxBegin = 0
		}
		if ctxEnd > len(data) {
			ctxEnd = len(data)
		}
		hits = append(hits, Hit{
			Word:    string(t.Word),
			Index:   t.Pos,
			Context: string(data[ctxBegin:ctxEnd]),
		})
	}
	return hits, nil
}
