package main

import (
	"flag"
	"io"
	"path"
	"strings"
	"sync"

	"fmt"
	"os"

	"path/filepath"

	"io/ioutil"

	"github.com/joelanford/goscan/utils/ahocorasick"
	"github.com/joelanford/goscan/utils/ramdisk"
	"github.com/joelanford/goscan/utils/unar"
	"github.com/joelanford/goscan/utils/words"
	"github.com/pkg/errors"
	"gopkg.in/h2non/filetype.v1"
)

type Opts struct {
	ScanFiles      []string
	RamdiskSize    int
	DirtyWordsFile string
	DbFile         string
}

var (
	rpmType = filetype.AddType("rpm", "application/x-rpm")
)

func init() {
	filetype.AddMatcher(rpmType, func(header []byte) bool {
		return len(header) >= 4 && header[0] == 0xED && header[1] == 0xAB && header[2] == 0xEE && header[3] == 0xDB
	})
}

func main() {
	opts, errs := parseFlags()
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "error(s) parsing flags:\n")
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", err.Error())
		}
		os.Exit(1)
	}

	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseFlags() (Opts, []error) {
	var errs []error
	var opts Opts
	flag.StringVar(&opts.DirtyWordsFile, "words", "", "YAML dirty words file")
	flag.StringVar(&opts.DbFile, "db", ".files.sqlite", "Database to track previously seen files")
	flag.IntVar(&opts.RamdiskSize, "ramdisk.size", 4096, "Size of ramdisk to use as scratch space")
	flag.Parse()
	opts.ScanFiles = flag.Args()

	if opts.DirtyWordsFile == "" {
		errs = append(errs, errors.New("words file not defined"))
	}

	if opts.DbFile == "" {
		errs = append(errs, errors.New("db file not defined"))
	}

	if len(opts.ScanFiles) == 0 {
		errs = append(errs, errors.New("scan paths not defined"))
	}

	return opts, errs
}

func run(opts Opts) error {
	dirtyWords, err := words.LoadFile(opts.DirtyWordsFile)
	if err != nil {
		return err
	}

	var dictionary []string
	for _, w := range dirtyWords {
		dictionary = append(dictionary, w.Word)
	}

	rd := ramdisk.New("goscan", opts.RamdiskSize)
	err = rd.Mount()
	if err != nil {
		return err
	}
	defer rd.Unmount()

	fileChan := make(chan string)
	var wg sync.WaitGroup
	wg.Add(16)
	for i := 0; i < 16; i++ {
		go func() {
			defer wg.Done()
			for file := range fileChan {
				hits, err := searchFile(file, dictionary)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					return
				}
				for _, hit := range hits {
					fmt.Println(hit)
				}
				os.Remove(file)
			}
		}()
	}
	for _, scanFile := range opts.ScanFiles {
		ifile, err := os.Open(scanFile)
		if err != nil {
			return err
		}
		ofilename := path.Join(rd.MountPoint(), path.Base(scanFile))
		ofile, err := os.Create(ofilename)
		if err != nil {
			return err
		}
		if _, err := io.Copy(ofile, ifile); err != nil {
			return err
		}
		if err := filepath.Walk(ofilename, explode(fileChan)); err != nil {
			return err
		}
	}
	close(fileChan)
	wg.Wait()
	return nil
}

func explode(fileChan chan<- string) filepath.WalkFunc {
	return func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && info.Size() > 0 {
			k, err := filetype.MatchFile(file)
			if err != nil {
				return err
			}

			if k.Extension == "zip" ||
				k.Extension == "gz" ||
				k.Extension == "xz" ||
				k.Extension == "bz2" ||
				k.Extension == "7z" ||
				k.Extension == "tar" ||
				k.Extension == "rar" ||
				k.Extension == "rpm" ||
				k.Extension == "deb" ||
				k.Extension == "pdf" ||
				k.Extension == "exe" ||
				k.Extension == "rtf" ||
				k.Extension == "ps" ||
				k.Extension == "cab" ||
				k.Extension == "ar" ||
				k.Extension == "Z" ||
				k.Extension == "lz" ||
				strings.HasSuffix(file, ".cpio") ||
				strings.HasSuffix(file, ".iso") ||
				strings.HasSuffix(file, ".img") {
				explodePath := file + ".unar"
				if err := unar.Run(file, explodePath); err != nil {
					fileChan <- file
					if strings.HasSuffix(err.Error(), "Couldn't recognize the archive format.") {
						return nil
					}
					fmt.Fprintf(os.Stderr, "error unarchiving file: %s\n", file)
					return nil
				}
				fileChan <- file
				if _, err := os.Stat(explodePath); !os.IsNotExist(err) {
					if err := filepath.Walk(explodePath, explode(fileChan)); err != nil {
						return err
					}
				}
			} else {
				fileChan <- file
			}
		}
		return nil
	}
}

func searchFile(searchFile string, dirtyWords []string) ([]words.Hit, error) {
	m := new(ahocorasick.Machine)
	dict := [][]byte{}
	for _, w := range dirtyWords {
		dict = append(dict, []byte(w))
	}
	m.Build(dict)

	data, err := ioutil.ReadFile(searchFile)
	if err != nil {
		return nil, err
	}
	terms := m.MultiPatternSearch(data, false)

	hits := []words.Hit{}
	for _, t := range terms {
		ctxBegin := t.Pos - 20
		ctxEnd := t.Pos + len(t.Word) + 20
		if ctxBegin < 0 {
			ctxBegin = 0
		}
		if ctxEnd > len(data) {
			ctxEnd = len(data)
		}
		hits = append(hits, words.Hit{
			Word:      string(t.Word),
			File:      searchFile,
			FileIndex: t.Pos,
			Context:   string(data[ctxBegin:ctxEnd]),
		})
	}
	return hits, nil
}
