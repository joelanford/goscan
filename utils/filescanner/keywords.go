package filescanner

import (
	"io/ioutil"
	"strings"

	"github.com/joelanford/goscan/utils/ahocorasick"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type Keywords struct {
	keywords   map[string]*Keyword
	dictionary *ahocorasick.Machine
}

type Keyword struct {
	Word     string            `yaml:"word"`
	Policies map[string]string `yaml:"policies"`
}

func LoadKeywords(wordsFile string, filterPolicies []string) (*Keywords, error) {
	//
	// Get keywords from file
	//
	data, err := ioutil.ReadFile(wordsFile)
	if err != nil {
		return nil, errors.Wrap(err, "error reading keywords file")
	}

	var keywordList []*Keyword
	err = yaml.Unmarshal(data, &keywordList)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing keywords file")
	}

	//
	// Build map of keywords, filtering them by specified policy
	//
	keywords := make(map[string]*Keyword)
	for _, keyword := range keywordList {
		if filterPolicies == nil {
			keywords[keyword.Word] = keyword
		} else {
			kwPolicies := make(map[string]string)
			for _, filterPolicy := range filterPolicies {
				if p, ok := keyword.Policies[filterPolicy]; ok {
					kwPolicies[filterPolicy] = p
				}
			}
			if len(kwPolicies) > 0 || keyword.Policies == nil || len(keyword.Policies) == 0 {
				keyword.Policies = kwPolicies
				keywords[keyword.Word] = keyword
			}
		}
	}

	//
	// If provided filter policies did not match any keywords, report error
	//
	if len(keywords) == 0 {
		return nil, errors.Errorf("no keywords matched policy filter: %s", strings.Join(filterPolicies, ","))
	}

	//
	// Create the Aho-Corasick dictionary for fast string matching
	//
	var keywordsBytes [][]byte
	for _, keyword := range keywords {
		keywordsBytes = append(keywordsBytes, []byte(keyword.Word))
	}
	dictionary := &ahocorasick.Machine{}
	dictionary.Build(keywordsBytes)

	return &Keywords{
		keywords:   keywords,
		dictionary: dictionary}, nil
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
			Word:     string(t.Word),
			Index:    t.Pos,
			Context:  string(data[ctxBegin:ctxEnd]),
			Policies: k.keywords[string(t.Word)].Policies,
		})
	}
	return hits, nil
}
