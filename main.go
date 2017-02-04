package main

import (
	"flag"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"fmt"
	"os"

	"github.com/joelanford/goscan/utils/ramdisk"
	"github.com/joelanford/goscan/utils/unar"
	"github.com/pkg/errors"
)

var (
	scanFiles []string
	wordsFile string
	dbFile    string
)

func main() {
	if errs := parseFlags(); len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "error(s) parsing flags:\n")
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", err.Error())
		}
		os.Exit(1)
	}

	for _, file := range scanFiles {
		err := scanArchiveFile(file, wordsFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func parseFlags() []error {
	var errs []error
	flag.StringVar(&wordsFile, "words", "", "File containing strings to match, one per line")
	flag.StringVar(&dbFile, "db", ".files.sqlite", "Database to track previously seen files")
	flag.Parse()
	scanFiles = flag.Args()

	if wordsFile == "" {
		errs = append(errs, errors.New("words file not defined"))
	}

	if dbFile == "" {
		errs = append(errs, errors.New("db file not defined"))
	}

	if len(scanFiles) == 0 {
		errs = append(errs, errors.New("scan files not defined"))
	}

	return errs
}

func scanArchiveFile(file, wordsFile string) error {
	rd := ramdisk.New("goscan", 4096)
	err := rd.Mount()
	if err != nil {
		return err
	}
	defer rd.Unmount()

	err = unar.Run(file, rd.MountPoint())
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	fileChan := make(chan string)

	wg.Add(16)
	for i := 0; i < 16; i++ {
		go func() {
			for file := range fileChan {
				grepFile(file, wordsFile)
			}
			wg.Done()
		}()
	}

	filepath.Walk(rd.MountPoint(), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "SKIP: error walking directory tree: %s\n", err)
			return nil
		}
		if !info.IsDir() {
			fileChan <- path
		}
		return nil
	})
	close(fileChan)

	wg.Wait()

	return nil
}

func grepFile(searchFile string, patternFile string) error {
	out, err := exec.Command("grep", "-H", "--text", "-n", "-F", "-i", "-f", patternFile, searchFile).CombinedOutput()
	if err != nil {
		if eerr, ok := err.(*exec.ExitError); ok {
			if werr, ok := eerr.Sys().(*syscall.WaitStatus); ok {
				if werr.ExitStatus() > 1 {
					return errors.Wrapf(err, "error grepping file %s", searchFile)
				}
			}
		}
	}
	fmt.Print(string(out))
	return nil
}
