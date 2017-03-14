package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joelanford/goscan/app"
	"github.com/joelanford/goscan/app/goscan"
	"github.com/joelanford/goscan/utils/keywords"
	"github.com/joelanford/goscan/utils/output"
	"github.com/joelanford/goscan/utils/scratch"
	"github.com/pkg/errors"
)

func Run(opts *app.Opts) error {
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
	ss := scratch.New(opts.BaseDir)
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
			if err != context.Canceled {
				return errors.Wrapf(err, "error scanning file")
			}
			return nil
		case sr, ok := <-scanResults:
			if !ok {
				sum.Stats.Duration = time.Now().Sub(start).Seconds()
				w.WriteSummary(sum)
				return nil
			}
			sum.Stats.FilesScanned++
			if !opts.HitsOnly || len(sr.Hits) > 0 {
				sum.Results = append(sum.Results, sr)
				if len(sr.Hits) > 0 {
					sum.Stats.FilesHit++
					sum.Stats.TotalHits += len(sr.Hits)
				}
			}
		}
	}
}

func setupSignalCancellationContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGABRT, syscall.SIGINT, syscall.SIGKILL)
	go func() {
		sig := <-sigChan
		fmt.Fprintf(os.Stderr, "Received signal %s. Exiting\n", sig)
		cancel()
	}()
	return ctx
}
