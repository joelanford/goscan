package filescanner

import (
	"io/ioutil"

	"github.com/joelanford/goscan/utils/ahocorasick"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type DirtyWords struct {
	Keywords []Keyword `yaml:"keywords"`

	dictionary *ahocorasick.Machine
}

type Keyword struct {
	Word      string   `yaml:"word"`
	Blacklist []string `yaml:"blacklist"`
	Reason    string   `yaml:"reason"`
}

type Hit struct {
	Word    string `json:"word"`
	File    string `json:"file"`
	Index   int    `json:"index"`
	Context string `json:"context"`
}

func LoadDirtyWords(wordsFile string) (*DirtyWords, error) {
	data, err := ioutil.ReadFile(wordsFile)
	if err != nil {
		return nil, errors.Wrap(err, "error reading dirty words file")
	}

	var dirtyWords DirtyWords
	err = yaml.Unmarshal(data, &dirtyWords)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing dirty words file")
	}

	var keywords [][]byte
	for _, keyword := range dirtyWords.Keywords {
		keywords = append(keywords, []byte(keyword.Word))
	}

	dirtyWords.dictionary = &ahocorasick.Machine{}
	dirtyWords.dictionary.Build(keywords)

	return &dirtyWords, nil
}

func (d *DirtyWords) MatchFile(file string) ([]Hit, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var hits []Hit
	for _, t := range d.dictionary.MultiPatternSearch(data, false) {
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
			File:    file,
			Index:   t.Pos,
			Context: string(data[ctxBegin:ctxEnd]),
		})
	}
	return hits, nil
}
