package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joelanford/goscan/utils/archive"
	"github.com/joelanford/goscan/utils/keywords"
	"github.com/joelanford/goscan/utils/output"
	"github.com/joelanford/goscan/utils/scratch"
)

type Opts struct {
	BaseDir       string
	InputFiles    []string
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
		return err
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
		InputFiles: opts.InputFiles,
		Results:    make([]output.ScanResult, 0),
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
		return err
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
			return err
		}
	}
	defer outputFile.Close()

	//
	// Prepare the scratch space
	//
	ss := scratch.New(opts.BaseDir, opts.RamdiskEnable, opts.RamdiskSize)
	err = ss.Setup()
	if err != nil {
		return err
	}
	defer ss.Teardown()

	//
	// errChan used for the following goroutines to be able to return an error
	// and shortcircuit the rest of the processing.
	//
	errChan := make(chan error)

	//
	// Copy the files to be scanned into the scratch space
	//
	ifiles := make(chan string)
	go func() {
		defer close(ifiles)
		for _, ifile := range opts.InputFiles {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
				ofile, err := ss.CopyFile(ifile)
				if err != nil {
					errChan <- err
					return
				}
				ifiles <- ofile
			}
		}
	}()

	//
	// Recursively unarchive the files to be scanned
	//
	var unarchiveWg sync.WaitGroup
	unarchiveResults := make(chan archive.UnarchiveResult)
	unarchiveWg.Add(len(opts.InputFiles))
	go func() {
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case ifile, ok := <-ifiles:
				if !ok {
					return
				}
				go func() {
					archive.UnarchiveRecursive(ctx, ifile, ".goscan-unar", unarchiveResults)
					unarchiveWg.Done()
				}()
			}
		}
	}()

	go func() {
		unarchiveWg.Wait()
		close(unarchiveResults)
	}()

	//
	// Scan unarchived files for hits
	//
	var scanWg sync.WaitGroup
	scanResults := make(chan output.ScanResult)
	scanWg.Add(opts.Parallelism)
	for i := 0; i < opts.Parallelism; i++ {
		go func() {
			defer scanWg.Done()
			for {
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				case ur, ok := <-unarchiveResults:
					if !ok {
						return
					}
					if ur.Error != nil {
						errChan <- ur.Error
						return
					}
					hits, err := kw.MatchFile(ur.File, opts.HitContext)
					if err != nil {
						errChan <- err
						return
					}
					scanResults <- output.ScanResult{
						File: strings.Replace(strings.Replace(ur.File, ss.Dir(), "", -1), ".goscan-unar", "", -1),
						Hits: hits,
					}

					sum.Stats.FilesScanned++
					if len(hits) <= 0 {
						sum.Stats.FilesHit++
					}

				}
			}
		}()
	}

	go func() {
		scanWg.Wait()
		close(scanResults)
	}()

	//
	// Loop until error or all hits have been found
	//
	for {
		select {
		case err = <-errChan:
			return err
		case sr, ok := <-scanResults:
			if !ok {
				sum.Stats.Duration = time.Now().Sub(start).Seconds()

				w.WriteSummary(sum)
				return nil
			}
			if !opts.HitsOnly || len(sr.Hits) > 0 {
				sum.Results = append(sum.Results, sr)
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
	flag.StringVar(&opts.ResultsFile, "outputFile", "-", "Results output file (\"-\" for stdout)")
	flag.StringVar(&opts.ResultsFormat, "outputFormat", "json", "Results output format")
	flag.IntVar(&opts.Parallelism, "parallelism", runtime.NumCPU(), "Number of goroutines to use to scan files")
	configureRamdiskOpts(&opts)

	flag.Parse()

	opts.InputFiles = flag.Args()

	if opts.RamdiskEnable && opts.RamdiskSize < 0 {
		return nil, errors.New("error: ramdisk.size must be positive")
	}

	if opts.KeywordsFile == "" {
		return nil, errors.New("error: scan.words file must be defined")
	}

	if opts.HitContext < 0 {
		return nil, errors.New("error: scan.context must not be negative")
	}

	if policies == "all" {
		opts.Policies = nil
	} else {
		opts.Policies = strings.Split(policies, ",")
	}

	if opts.Parallelism < 1 {
		return nil, errors.New("error: scan.parallelism must be positive")
	}

	if len(opts.InputFiles) == 0 {
		return nil, errors.New("error: must define at least one file to scan")
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
