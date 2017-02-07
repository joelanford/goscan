package main

import (
	"context"
	"encoding/json"
	"flag"
	"math/rand"
	"path"

	"fmt"
	"os"

	"os/signal"
	"syscall"

	"github.com/joelanford/goscan/utils/filescanner"
	"github.com/joelanford/goscan/utils/ramdisk"
	"github.com/pkg/errors"
	"gopkg.in/h2non/filetype.v1"
)

type FileOpts struct {
	ScanFiles   []string
	ResultsFile string
}

var (
	rpmType = filetype.AddType("rpm", "application/x-rpm")
)

func init() {
	filetype.AddMatcher(rpmType, func(header []byte) bool {
		return len(header) >= 4 && header[0] == 0xED && header[1] == 0xAB && header[2] == 0xEE && header[3] == 0xDB
	})

	flag.Usage = func() {
		fmt.Printf("Usage: goscan [options] <scanfiles>\n")
		flag.PrintDefaults()
	}
}

func exit(err error, code int, rd *ramdisk.Ramdisk) {
	if rd != nil {
		if err := rd.Unmount(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(code)
}

func main() {
	//
	// Parse command line flags
	//
	scanOpts, fileOpts, ramdiskOpts, errs := parseFlags()
	if len(errs) > 0 {
		e := "error(s) parsing flags:\n"
		for _, err := range errs {
			e = fmt.Sprintf("%s  - %s\n", e, err.Error())
		}
		exit(errors.New(e), 1, nil)
	}

	//
	// Prepare the ramdisk
	//
	rd := ramdisk.New(ramdiskOpts)
	err := rd.Mount()
	if err != nil {
		exit(err, 1, nil)

	}
	defer rd.Unmount()

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
	// Setup the filescanner
	//
	fs, err := filescanner.New(scanOpts)
	if err != nil {
		exit(err, 1, rd)
	}

	//
	// Run the scan
	//
	resChan := make(chan filescanner.ScanResult)
	err = fs.Scan(ctx, resChan, fileOpts.ScanFiles...)
	if err != nil {
		exit(err, 1, rd)
	}

	//
	// Output the hits
	//
	output, err := os.Create(fileOpts.ResultsFile)
	if err != nil {
		exit(err, 1, rd)
	}
	e := json.NewEncoder(output)
	for result := range resChan {
		err := e.Encode(result)
		if err != nil {
			exit(err, 1, rd)
		}
	}
}

func parseFlags() (filescanner.Opts, FileOpts, ramdisk.Opts, []error) {
	var errs []error

	var scanOpts filescanner.Opts
	flag.StringVar(&scanOpts.DirtyWordsFile, "scan.words", "", "YAML dirty words file")
	flag.IntVar(&scanOpts.HitContext, "scan.context", 10, "Context to capture around each hit")
	flag.StringVar(&scanOpts.ScratchSpacePath, "scan.scratch.dir", "", "Scratch directory for scan unarchiving")
	if scanOpts.ScratchSpacePath == "" {
		scanOpts.ScratchSpacePath = fmt.Sprintf("/tmp/goscan-%d", rand.Int())
	}

	var ramdiskOpts ramdisk.Opts
	flag.IntVar(&ramdiskOpts.Megabytes, "ramdisk.mb", 4096, "Size of ramdisk to use as scratch space")
	ramdiskOpts.Name = path.Base(scanOpts.ScratchSpacePath)
	ramdiskOpts.MountPoint = scanOpts.ScratchSpacePath

	var fileOpts FileOpts
	flag.StringVar(&fileOpts.ResultsFile, "output", "-", "Results output file (\"-\" for stdout)")
	if fileOpts.ResultsFile == "-" {
		fileOpts.ResultsFile = "/dev/stdout"
	}

	flag.Parse()
	fileOpts.ScanFiles = flag.Args()

	if scanOpts.DirtyWordsFile == "" {
		errs = append(errs, errors.New("words file not defined"))
	}

	if len(fileOpts.ScanFiles) == 0 {
		errs = append(errs, errors.New("scan files not defined"))
	}
	return scanOpts, fileOpts, ramdiskOpts, errs
}
