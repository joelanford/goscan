package output

import "github.com/joelanford/goscan/utils/keywords"

type ScanSummary struct {
	InputFiles []string     `json:"inputFiles" yaml:"inputFiles"`
	Results    []ScanResult `json:"results" yaml:"results"`
	Stats      ScanStats    `json:"stats" yaml:"stats"`
}

type ScanResult struct {
	File string         `json:"file" yaml:"file"`
	Hits []keywords.Hit `json:"hits" yaml:"hits`
}

type ScanStats struct {
	FilesScanned int     `json:"filesScanned" yaml:"filesScanned"`
	FilesHit     int     `json:"filesHit" yaml:"filesHit"`
	TotalHits    int     `json:"totalHits" yaml:"totalHits"`
	Duration     float64 `json:"duration" yaml:"duration"`
}

type SummaryWriter interface {
	WriteSummary(ScanSummary) error
}
