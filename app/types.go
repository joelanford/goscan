package app

import "github.com/joelanford/goscan/utils/keywords"

type ScanOpts struct {
	BaseDir      string
	InputFiles   []string
	KeywordsFile string
	Policies     []string
	HitContext   int
	HitsOnly     bool
	ResultsFile  string
	Parallelism  int

	RamdiskEnable bool
	RamdiskSize   int
}

type ScanResult struct {
	File string         `json:"file"`
	Hits []keywords.Hit `json:"hits"`
}
