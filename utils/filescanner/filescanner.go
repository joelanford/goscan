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
	DirtyWordsFile   string
	HitContext       int
	ScratchSpacePath string
}

type FileScanner struct {
	keywords         *DirtyWords
	dbFile           string
	scratchSpacePath string
	hitContext       int
}

type WalkResult struct {
	File  string `json:"file"`
	Error error  `json:"error,omitempty"`
}

type ScanResult struct {
	File  string `json:"file"`
	Hits  []Hit  `json:"hits"`
	Error error  `json:"error,omitempty"`
}

type Hit struct {
	Word    string `json:"word"`
	Index   int    `json:"index"`
	Context string `json:"context"`
}

func New(opts Opts) (*FileScanner, error) {
	dirtyWords, err := LoadDirtyWords(opts.DirtyWordsFile)
	if err != nil {
		return nil, errors.Wrap(err, "error loading dirty words")
	}

	return &FileScanner{
		keywords:         dirtyWords,
		scratchSpacePath: opts.ScratchSpacePath,
		hitContext:       opts.HitContext,
	}, nil
}

func (fs *FileScanner) Scan(ctx context.Context, scanResultsChan chan<- ScanResult, scanFiles ...string) error {
	if len(scanFiles) == 0 {
		close(scanResultsChan)
		return nil
	}

	inFileChan := make(chan string, len(scanFiles))
	for _, file := range scanFiles {
		//
		// Copy the file to scan into the scratch space
		//
		ifile, err := os.Open(file)
		if err != nil {
			return err
		}
		ofilename := path.Join(fs.scratchSpacePath, path.Base(file))
		ofile, err := os.Create(ofilename)
		if err != nil {
			return err
		}
		if _, err := io.Copy(ofile, ifile); err != nil {
			return err
		}

		inFileChan <- ofilename
	}
	close(inFileChan)

	walkResultsChan := make(chan WalkResult)
	walkStarted := make(chan struct{})
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
				close(walkStarted)
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
		<-walkStarted
		walkWg.Wait()
		close(walkResultsChan)
	}()

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
					sr.File = strings.Replace(strings.Replace(wr.File, ".goscan-unar", "", -1), fs.scratchSpacePath+"/", "", -1)
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
	return nil
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

			explodePath := file + ".goscan-unar"
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
