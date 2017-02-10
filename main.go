package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"path"
	"runtime"
	"strings"
	"sync"

	"fmt"
	"os"

	"os/signal"
	"syscall"

	"github.com/joelanford/goscan/utils/archive"
	"github.com/joelanford/goscan/utils/keywords"
	"github.com/joelanford/goscan/utils/scratch"
	"github.com/pkg/errors"
)

type ScanOpts struct {
	InputFiles   []string
	KeywordsFile string
	Policies     []string
	HitContext   int
	ResultsFile  string
	Parallelism  int
}

type ScanResult struct {
	File string         `json:"file"`
	Hits []keywords.Hit `json:"hits"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	//
	// Parse command line flags
	//
	scratchOpts, scanOpts, err := parseFlags()
	if err != nil {
		return err
	}

	//
	// Setup context and signal handlers, which will be needed
	// if we need to cleanly exit before completing the scan.
	//
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGABRT, syscall.SIGINT, syscall.SIGKILL)
	go func() {
		sig := <-sigChan
		fmt.Fprintf(os.Stderr, "Received signal %s. Exiting", sig)
		cancel()
	}()

	//
	// Setup the keyword matcher
	//
	kw, err := keywords.Load(scanOpts.KeywordsFile, scanOpts.Policies)
	if err != nil {
		return err
	}

	//
	// Open the output file
	//
	var output io.WriteCloser
	if scanOpts.ResultsFile == "-" {
		output = os.Stdout
	} else {
		output, err = os.Create(scanOpts.ResultsFile)
		if err != nil {
			return err
		}
	}
	defer output.Close()
	e := json.NewEncoder(output)

	//
	// Prepare the scratch space
	//
	ss := scratch.New(*scratchOpts)
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
		for _, ifile := range scanOpts.InputFiles {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
				ofile := path.Join(ss.ScratchSpacePath, path.Base(ifile))
				if err := copyToScratchSpace(ifile, ofile); err != nil {
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
	unarchiveWg.Add(len(scanOpts.InputFiles))
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
	scanResults := make(chan ScanResult)
	scanWg.Add(scanOpts.Parallelism)
	for i := 0; i < scanOpts.Parallelism; i++ {
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
					hits, err := kw.MatchFile(ur.File, scanOpts.HitContext)
					if err != nil {
						errChan <- err
						return
					}
					scanResults <- ScanResult{
						File: strings.Replace(strings.Replace(ur.File, ss.ScratchSpacePath+"/", "", -1), ".goscan-unar", "", -1),
						Hits: hits,
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
				return nil
			}
			err = e.Encode(sr)
			if err != nil {
				return err
			}
		}
	}
}

func parseFlags() (*scratch.Opts, *ScanOpts, error) {
	flag.Usage = func() {
		fmt.Printf("Usage: goscan [options] <scanfiles>\n")
		flag.PrintDefaults()
	}

	var scratchOpts scratch.Opts
	var policies string
	var scanOpts ScanOpts

	configureScratchOpts(&scratchOpts)
	flag.StringVar(&scanOpts.KeywordsFile, "scan.words", "", "YAML keywords file")
	flag.IntVar(&scanOpts.HitContext, "scan.context", 10, "Context to capture around each hit")
	flag.StringVar(&policies, "scan.policies", "all", "Comma-separated list of keyword policies")
	flag.StringVar(&scanOpts.ResultsFile, "scan.output", "-", "Results output file (\"-\" for stdout)")
	flag.IntVar(&scanOpts.Parallelism, "scan.parallelism", runtime.NumCPU(), "Number of goroutines to use to scan files.")

	flag.Parse()

	scanOpts.InputFiles = flag.Args()

	if scratchOpts.RamdiskEnable && scratchOpts.RamdiskMegabytes < 0 {
		return nil, nil, errors.New("error: scratch.ramdisk.mb must be positive")
	}

	if scanOpts.KeywordsFile == "" {
		return nil, nil, errors.New("error: scan.words file must be defined")
	}

	if scanOpts.HitContext < 0 {
		return nil, nil, errors.New("error: scan.context must not be negative")
	}

	if policies == "all" {
		scanOpts.Policies = nil
	} else {
		scanOpts.Policies = strings.Split(policies, ",")
	}

	if scanOpts.Parallelism < 1 {
		return nil, nil, errors.New("error: scan.parallelism must be positive")
	}

	if len(scanOpts.InputFiles) == 0 {
		return nil, nil, errors.New("error: must define at least one file to scan")
	}
	return &scratchOpts, &scanOpts, nil
}

func copyToScratchSpace(ifilename, ofilename string) error {
	ifile, err := os.Open(ifilename)
	if err != nil {
		return err
	}
	defer ifile.Close()
	ofile, err := os.Create(ofilename)
	if err != nil {
		return err
	}
	defer ofile.Close()
	if _, err := io.Copy(ofile, ifile); err != nil {
		return err
	}
	return nil
}
