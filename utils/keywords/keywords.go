package keywords

import (
	"io"
	"io/ioutil"
	"os"
	"sort"
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

type Hit struct {
	Word     string            `json:"word"`
	Index    int               `json:"index"`
	Context  string            `json:"context"`
	Policies map[string]string `json:"policies,omitempty"`
}

func LoadReader(r io.Reader, policies []string) (*Keywords, error) {
	//
	// Get keywords from file
	//
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, errors.Wrap(err, "error reading keywords")
	}

	var keywordList []*Keyword
	err = yaml.Unmarshal(data, &keywordList)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing keywords")
	}

	//
	// Build map of keywords, filtering them by specified policy
	//
	keywords := make(map[string]*Keyword)
	for _, keyword := range keywordList {
		if policies == nil {
			keywords[keyword.Word] = keyword
		} else {
			kwPolicies := make(map[string]string)
			for _, policy := range policies {
				if p, ok := keyword.Policies[policy]; ok {
					kwPolicies[policy] = p
				}
			}
			if len(kwPolicies) > 0 || keyword.Policies == nil || len(keyword.Policies) == 0 {
				keyword.Policies = kwPolicies
				keywords[keyword.Word] = keyword
			}
		}
	}

	//
	// If provided filter policies did not match any keywords, return error
	//
	if len(keywords) == 0 {
		return nil, errors.Errorf("no keywords matched policy filter: %s", strings.Join(policies, ","))
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
		dictionary: dictionary,
	}, nil
}

func LoadFile(wordsFile string, policies []string) (*Keywords, error) {
	r, err := os.Open(wordsFile)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening keyword file %s", wordsFile)
	}
	defer r.Close()
	return LoadReader(r, policies)
}

func (k *Keywords) Keywords() []Keyword {
	kwSlice := []Keyword{}
	for _, v := range k.keywords {
		kwSlice = append(kwSlice, *v)
	}
	sort.Slice(kwSlice, func(i, j int) bool {
		return kwSlice[i].Word < kwSlice[j].Word
	})
	return kwSlice
}

func (k *Keywords) MatchFile(file string, hitContext int) ([]Hit, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	hits := make([]Hit, 0)
	terms, err := k.dictionary.MultiPatternSearchReadSeeker(f, hitContext, false)
	for _, t := range terms {
		hits = append(hits, Hit{
			Word:     string(t.Word),
			Index:    t.Pos,
			Context:  string(t.Context),
			Policies: k.keywords[string(t.Word)].Policies,
		})
	}
	return hits, err
}
