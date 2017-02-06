package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
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

var (
	rpmType = filetype.AddType("rpm", "application/x-rpm")
)

func init() {
	filetype.AddMatcher(rpmType, func(header []byte) bool {
		return len(header) >= 4 && header[0] == 0xED && header[1] == 0xAB && header[2] == 0xEE && header[3] == 0xDB
	})
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
	scanOpts, ramdiskOpts, errs := parseFlags()
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
	scanOpts.ScratchSpacePath = rd.MountPoint()
	fs, err := filescanner.New(scanOpts)
	if err != nil {
		exit(err, 1, rd)
	}

	//
	// Run the scan
	//
	results, err := fs.Scan(ctx)
	if err != nil {
		exit(err, 1, rd)
	}

	//
	// Output the hits
	//
	data, err := json.MarshalIndent(results, "", "    ")
	ioutil.WriteFile(scanOpts.ResultsFile, data, 0644)
}

func parseFlags() (filescanner.Opts, ramdisk.Opts, []error) {
	var errs []error

	var scanOpts filescanner.Opts
	flag.StringVar(&scanOpts.DirtyWordsFile, "scan.words", "", "YAML dirty words file")
	flag.StringVar(&scanOpts.DbFile, "scan.db", path.Join(os.Getenv("HOME"), "goscan.sqlite"), "Database to track previously seen files")
	flag.StringVar(&scanOpts.ResultsFile, "scan.results", "-", "Results output file (\"-\" for stdout)")
	if scanOpts.ResultsFile == "-" {
		scanOpts.ResultsFile = "/dev/stdout"
	}

	var ramdiskOpts ramdisk.Opts
	flag.StringVar(&ramdiskOpts.Name, "ramdisk.name", "goscan", "Disk label to use for ramdisk")
	flag.StringVar(&ramdiskOpts.BaseDir, "ramdisk.basedir", "/tmp", "Base directory for ramdisk mountpoints")
	flag.IntVar(&ramdiskOpts.Megabytes, "ramdisk.megabytes", 4096, "Size of ramdisk to use as scratch space")

	flag.Parse()
	scanOpts.ScanFiles = flag.Args()

	if scanOpts.DirtyWordsFile == "" {
		errs = append(errs, errors.New("words file not defined"))
	}

	if scanOpts.DbFile == "" {
		errs = append(errs, errors.New("db file not defined"))
	}

	if len(scanOpts.ScanFiles) == 0 {
		errs = append(errs, errors.New("scan files not defined"))
	}

	return scanOpts, ramdiskOpts, errs
}

// func run(ctx context.Context, opts filescanner.Opts, scratchSpacePath string) error {
// 	dirtyWords, err := dirtywords.FromFile(opts.DirtyWordsFile)
// 	if err != nil {
// 		return err
// 	}

// 	fileChan := make(chan string)
// 	var wg sync.WaitGroup
// 	wg.Add(16)
// 	for i := 0; i < 16; i++ {
// 		go func() {
// 			defer wg.Done()
// 			for {
// 				select {
// 				case <-ctx.Done():
// 					return
// 				case file, ok := <-fileChan:
// 					if !ok {
// 						return
// 					}
// 					hits, err := dirtyWords.MatchFile(file)
// 					if err != nil {
// 						fmt.Fprintln(os.Stderr, err)
// 						return
// 					}
// 					for _, hit := range hits {
// 						fmt.Println(hit)
// 					}
// 					os.Remove(file)
// 				}
// 			}
// 		}()
// 	}
// 	for _, scanFile := range opts.ScanFiles {
// 		ifile, err := os.Open(scanFile)
// 		if err != nil {
// 			return err
// 		}
// 		ofilename := path.Join(scratchSpacePath, path.Base(scanFile))
// 		ofile, err := os.Create(ofilename)
// 		if err != nil {
// 			return err
// 		}
// 		if _, err := io.Copy(ofile, ifile); err != nil {
// 			return err
// 		}
// 		if err := filepath.Walk(ofilename, explode(fileChan)); err != nil {
// 			return err
// 		}
// 	}
// 	close(fileChan)
// 	wg.Wait()
// 	return nil
// }

// func explode(fileChan chan<- string) filepath.WalkFunc {
// 	return func(file string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			return err
// 		}
// 		if info.Mode().IsRegular() && info.Size() > 0 {
// 			k, err := filetype.MatchFile(file)
// 			if err != nil {
// 				return err
// 			}

// 			if k.Extension == "zip" ||
// 				k.Extension == "gz" ||
// 				k.Extension == "xz" ||
// 				k.Extension == "bz2" ||
// 				k.Extension == "7z" ||
// 				k.Extension == "tar" ||
// 				k.Extension == "rar" ||
// 				k.Extension == "rpm" ||
// 				k.Extension == "deb" ||
// 				k.Extension == "pdf" ||
// 				k.Extension == "exe" ||
// 				k.Extension == "rtf" ||
// 				k.Extension == "ps" ||
// 				k.Extension == "cab" ||
// 				k.Extension == "ar" ||
// 				k.Extension == "Z" ||
// 				k.Extension == "lz" ||
// 				strings.HasSuffix(file, ".cpio") ||
// 				strings.HasSuffix(file, ".iso") ||
// 				strings.HasSuffix(file, ".img") {
// 				explodePath := file + ".unar"
// 				if err := unar.Run(file, explodePath); err != nil {
// 					fileChan <- file
// 					if strings.HasSuffix(err.Error(), "Couldn't recognize the archive format.") {
// 						return nil
// 					}
// 					fmt.Fprintf(os.Stderr, "error unarchiving file: %s: %s\n", file, err)
// 					return nil
// 				}
// 				fileChan <- file
// 				if _, err := os.Stat(explodePath); !os.IsNotExist(err) {
// 					if err := filepath.Walk(explodePath, explode(fileChan)); err != nil {
// 						return err
// 					}
// 				}
// 			} else {
// 				fileChan <- file
// 			}
// 		}
// 		return nil
// 	}
// }
