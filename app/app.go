package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/joelanford/goscan/app/goscan"
	"github.com/joelanford/goscan/utils/keywords"
	"github.com/joelanford/goscan/utils/output"
	"github.com/joelanford/goscan/utils/scratch"
	"github.com/pkg/errors"
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

	RamdiskEnable bool
	RamdiskSize   int
}

func Run() error {
	//
	// Parse command line flags
	//
	opts, err := parseFlags()
	if err != nil {
		return errors.Wrapf(err, "error parsing flags")
	}

	//
	// Setup output formatter
	//
	var w output.SummaryWriter
	switch opts.ResultsFormat {
	case "json":
		w = output.NewJSONSummaryWriter(os.Stdout, "", "  ")
	case "yaml":
		w = output.NewYAMLSummaryWriter(os.Stdout)
	default:
		return errors.New("invalid results format")
	}

	sum := output.ScanSummary{
		InputFile: opts.InputFile,
		Results:   make([]output.ScanResult, 0),
	}
	start := time.Now()

	//
	// Setup context and signal handlers, which will be needed
	// if we need to cleanly exit before completing the scan.
	//
	ctx := setupSignalCancellationContext()

	//
	// Setup the keyword matcher
	//
	kw, err := keywords.Load(opts.KeywordsFile, opts.Policies)
	if err != nil {
		return errors.Wrapf(err, "error loading keywords")
	}

	//
	// Open the output file
	//
	var outputFile io.WriteCloser
	if opts.ResultsFile == "-" {
		outputFile = os.Stdout
	} else {
		outputFile, err = os.Create(opts.ResultsFile)
		if err != nil {
			return errors.Wrapf(err, "error opening output file")
		}
	}
	defer outputFile.Close()

	//
	// Prepare the scratch space
	//
	ss := scratch.New(opts.BaseDir, opts.RamdiskEnable, opts.RamdiskSize)
	err = ss.Setup()
	if err != nil {
		return errors.Wrapf(err, "scratch setup failed")
	}
	defer ss.Teardown()

	//
	// Copy input file into scratch space
	//
	ofile, err := ss.CopyFile(opts.InputFile)
	if err != nil {
		return errors.Wrapf(err, "scratch file copy failed")
	}

	scanResults := make(chan output.ScanResult)
	errChan := make(chan error)
	scanner, err := goscan.NewScanner(kw, opts.Policies, goscan.BaseDir(opts.BaseDir), goscan.HitContext(opts.HitContext), goscan.HitsOnly(opts.HitsOnly), goscan.Parallelism(opts.Parallelism))
	if err != nil {
		return errors.Wrapf(err, "failed to initialize scanner")
	}

	err = scanner.ScanFile(ctx, ofile, scanResults, errChan)
	if err != nil {
		return errors.Wrapf(err, "failed scanning file %s", opts.InputFile)
	}

	//
	// Loop until error or all hits have been found
	//
	for {
		select {
		case err = <-errChan:
			return errors.Wrapf(err, "error scanning file")
		case sr, ok := <-scanResults:
			if !ok {
				sum.Stats.Duration = time.Now().Sub(start).Seconds()
				w.WriteSummary(sum)
				return nil
			}
			sum.Stats.FilesScanned++
			if !opts.HitsOnly || len(sr.Hits) > 0 {
				sum.Results = append(sum.Results, sr)
				sum.Stats.FilesHit++
				sum.Stats.TotalHits += len(sr.Hits)
			}
		}
	}
}

func parseFlags() (*Opts, error) {
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
	configureRamdiskOpts(&opts)

	flag.Parse()

	if opts.RamdiskEnable && opts.RamdiskSize < 0 {
		return nil, errors.New("ramdisk.size must be > 0")
	}

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
	} else {
		opts.InputFile = flag.Arg(0)
	}
	return &opts, nil
}

func setupSignalCancellationContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGABRT, syscall.SIGINT, syscall.SIGKILL)
	go func() {
		sig := <-sigChan
		fmt.Fprintf(os.Stderr, "Received signal %s. Exiting", sig)
		cancel()
	}()
	return ctx
}
