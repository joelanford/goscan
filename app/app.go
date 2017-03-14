package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
)

type Opts struct {
	BaseDir       string
	InputFile     string
	KeywordsFile  string
	Policies      []string
	HitContext    int
	HitsOnly      bool
	ResultsFile   string
	ResultsFormat string
	Parallelism   int
}

func ParseFlags() (*Opts, error) {
	flag.Usage = func() {
		fmt.Printf("Usage: goscan [options] <scanfiles>\n")
		flag.PrintDefaults()
	}

	var policies string
	var opts Opts

	flag.StringVar(&opts.BaseDir, "basedir", os.TempDir(), "Scratch directory for scan unarchiving")
	flag.StringVar(&opts.KeywordsFile, "words", "", "YAML keywords file")
	flag.IntVar(&opts.HitContext, "context", 10, "Context to capture around each hit")
	flag.BoolVar(&opts.HitsOnly, "hitsonly", false, "Only output results containing hits")
	flag.StringVar(&policies, "policies", "all", "Comma-separated list of keyword policies")
	flag.StringVar(&opts.ResultsFile, "output.file", "-", "Results output file (\"-\" for stdout)")
	flag.StringVar(&opts.ResultsFormat, "output.format", "json", "Results output format")
	flag.IntVar(&opts.Parallelism, "parallelism", runtime.NumCPU(), "Number of goroutines to use to scan files")

	flag.Parse()

	if opts.KeywordsFile == "" {
		return nil, errors.New("words file must be defined")
	}

	if opts.HitContext < 0 {
		return nil, errors.New("context must not be >= 0")
	}

	if policies == "all" {
		opts.Policies = nil
	} else {
		opts.Policies = strings.Split(policies, ",")
	}

	if opts.Parallelism < 1 {
		return nil, errors.New("parallelism must be > 0")
	}

	if len(flag.Args()) != 1 {
		return nil, errors.New("must define exactly one file to scan")
	}
	opts.InputFile = flag.Arg(0)
	return &opts, nil
}
