package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/joelanford/goscan/utils/archive"
	"github.com/joelanford/goscan/utils/keywords"
	"github.com/joelanford/goscan/utils/scratch"
)

func Run() error {
	//
	// Parse command line flags
	//
	scanOpts, err := parseFlags()
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
	ss := scratch.New(scanOpts.BaseDir, scanOpts.RamdiskEnable, scanOpts.RamdiskSize)
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
				ifiledir := path.Dir(ifile)
				var ofiledir string
				if path.IsAbs(ifiledir) {
					ofiledir = path.Clean(path.Join(ss.Dir(), strings.Replace(ifiledir, ":", "_", -1)))
				} else {
					cwd, err := os.Getwd()
					if err != nil {
						errChan <- ctx.Err()
						return
					}
					ofiledir = path.Clean(path.Join(ss.Dir(), strings.Replace(cwd, ":", "_", -1), ifiledir))
				}
				ofile := path.Join(ofiledir, path.Base(ifile))
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
						File: strings.Replace(strings.Replace(ur.File, ss.Dir(), "", -1), ".goscan-unar", "", -1),
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
			if !scanOpts.HitsOnly || len(sr.Hits) > 0 {
				err = e.Encode(sr)
				if err != nil {
					return err
				}
			}
		}
	}
}

func parseFlags() (*ScanOpts, error) {
	flag.Usage = func() {
		fmt.Printf("Usage: goscan [options] <scanfiles>\n")
		flag.PrintDefaults()
	}

	var policies string
	var scanOpts ScanOpts

	flag.StringVar(&scanOpts.BaseDir, "basedir", os.TempDir(), "Scratch directory for scan unarchiving")
	flag.StringVar(&scanOpts.KeywordsFile, "words", "", "YAML keywords file")
	flag.IntVar(&scanOpts.HitContext, "context", 10, "Context to capture around each hit")
	flag.BoolVar(&scanOpts.HitsOnly, "hitsonly", false, "Only output results containing hits")
	flag.StringVar(&policies, "policies", "all", "Comma-separated list of keyword policies")
	flag.StringVar(&scanOpts.ResultsFile, "output", "-", "Results output file (\"-\" for stdout)")
	flag.IntVar(&scanOpts.Parallelism, "parallelism", runtime.NumCPU(), "Number of goroutines to use to scan files")
	configureRamdiskOpts(&scanOpts)

	flag.Parse()

	scanOpts.InputFiles = flag.Args()

	if scanOpts.RamdiskEnable && scanOpts.RamdiskSize < 0 {
		return nil, errors.New("error: ramdisk.size must be positive")
	}

	if scanOpts.KeywordsFile == "" {
		return nil, errors.New("error: scan.words file must be defined")
	}

	if scanOpts.HitContext < 0 {
		return nil, errors.New("error: scan.context must not be negative")
	}

	if policies == "all" {
		scanOpts.Policies = nil
	} else {
		scanOpts.Policies = strings.Split(policies, ",")
	}

	if scanOpts.Parallelism < 1 {
		return nil, errors.New("error: scan.parallelism must be positive")
	}

	if len(scanOpts.InputFiles) == 0 {
		return nil, errors.New("error: must define at least one file to scan")
	}
	return &scanOpts, nil
}

func copyToScratchSpace(ifilename, ofilename string) error {
	ifile, err := os.Open(ifilename)
	if err != nil {
		return err
	}
	defer ifile.Close()
	err = os.MkdirAll(path.Dir(ofilename), 0777)
	if err != nil {
		return err
	}
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