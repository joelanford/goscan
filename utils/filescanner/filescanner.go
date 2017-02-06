package filescanner

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	filetype "gopkg.in/h2non/filetype.v1"

	"github.com/joelanford/goscan/utils/unar"
	"github.com/pkg/errors"
)

type Opts struct {
	ScanFiles        []string
	DirtyWordsFile   string
	DbFile           string
	ScratchSpacePath string
	ResultsFile      string
}

type FileScanner struct {
	keywords         *DirtyWords
	scanFiles        []string
	dbFile           string
	scratchSpacePath string
}

type WalkResult struct {
	File  string
	Error error
}

type ScanResult struct {
	File  string
	Hits  []Hit
	Error error
}

func New(opts Opts) (*FileScanner, error) {
	dirtyWords, err := LoadDirtyWords(opts.DirtyWordsFile)
	if err != nil {
		return nil, errors.Wrap(err, "error loading dirty words")
	}

	return &FileScanner{
		keywords:         dirtyWords,
		scanFiles:        opts.ScanFiles,
		dbFile:           opts.DbFile,
		scratchSpacePath: opts.ScratchSpacePath,
	}, nil
}

func (fs *FileScanner) Scan(ctx context.Context) ([]ScanResult, error) {
	inFileChan := make(chan string, len(fs.scanFiles))
	for _, file := range fs.scanFiles {
		//
		// Copy the file to scan into the scratch space
		//
		ifile, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		ofilename := path.Join(fs.scratchSpacePath, path.Base(file))
		ofile, err := os.Create(ofilename)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(ofile, ifile); err != nil {
			return nil, err
		}

		inFileChan <- ofilename
	}
	close(inFileChan)

	walkResultsChan := make(chan WalkResult)
	var walkWg sync.WaitGroup
	go func() {
		for {
			select {
			case <-ctx.Done():
				walkResultsChan <- WalkResult{Error: ctx.Err()}
				return
			case scanFile, ok := <-inFileChan:
				//
				// If the channel is closed, there's nothing left to scan.
				// So we'll return the hits.
				//
				if !ok {
					return
				}

				//
				// Recursively unarchive ofilename, and send all newly unarchived
				// files on the fileChan channel to be scanned.
				//
				walkWg.Add(1)
				go func() {
					if err := filepath.Walk(scanFile, fs.explodeFiles(ctx, &walkWg, walkResultsChan)); err != nil {
						walkResultsChan <- WalkResult{Error: err}
					}
					walkWg.Done()
				}()
			}
		}
	}()

	go func() {
		walkWg.Wait()
		close(walkResultsChan)
	}()

	scanResultsChan := make(chan ScanResult)
	var scanWg sync.WaitGroup
	scanWg.Add(16)
	for i := 0; i < 16; i++ {
		go func() {
			defer scanWg.Done()
			for wr := range walkResultsChan {
				if wr.Error != nil {
					scanResultsChan <- ScanResult{Error: wr.Error}
				} else {
					var sr ScanResult
					sr.File = wr.File
					sr.Hits, sr.Error = fs.keywords.MatchFile(wr.File)
					scanResultsChan <- sr
					os.Remove(wr.File)
				}
			}
		}()
	}

	go func() {
		scanWg.Wait()
		close(scanResultsChan)
	}()

	var scanResults []ScanResult
	for sr := range scanResultsChan {
		if sr.Error != nil {
			return nil, sr.Error
		}
		scanResults = append(scanResults, sr)
	}

	return scanResults, nil
}

func (fs *FileScanner) explodeFiles(ctx context.Context, walkWg *sync.WaitGroup, walkResultsChan chan<- WalkResult) filepath.WalkFunc {
	return func(file string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return nil
		}

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
			unar.Run(file, explodePath)
			walkResultsChan <- WalkResult{File: file}
			if _, err := os.Stat(explodePath); !os.IsNotExist(err) {
				walkWg.Add(1)
				go func() {
					if err := filepath.Walk(explodePath, fs.explodeFiles(ctx, walkWg, walkResultsChan)); err != nil {
						walkResultsChan <- WalkResult{Error: err}
					}
					walkWg.Done()
				}()
			}
		} else {
			walkResultsChan <- WalkResult{File: file}
		}

		return nil
	}
}
