package scanner

import (
	"context"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"

	"github.com/joelanford/goscan/utils/archive"
	"github.com/joelanford/goscan/utils/keywords"
	"github.com/joelanford/goscan/utils/output"
	"github.com/pkg/errors"
)

type Option func(*Scanner) error

func HitsOnly(hitsOnly bool) Option {
	return func(s *Scanner) error {
		s.hitsOnly = hitsOnly
		return nil
	}
}

func HitContext(hitContext int) Option {
	return func(s *Scanner) error {
		if hitContext < 0 {
			return errors.New("error: hit context must be >= 0")
		}
		s.hitContext = hitContext
		return nil
	}
}

func BaseDir(baseDir string) Option {
	return func(s *Scanner) error {
		s.baseDir = baseDir
		return nil
	}
}

func Parallelism(parallelism int) Option {
	return func(s *Scanner) error {
		if parallelism < 1 {
			return errors.New("error: parallelism must be > 0")
		}
		s.parallelism = parallelism
		return nil
	}
}

type Scanner struct {
	keywords *keywords.Keywords

	hitsOnly    bool
	hitContext  int
	baseDir     string
	parallelism int
}

func NewScanner(keywords *keywords.Keywords, opts ...Option) (*Scanner, error) {
	s := &Scanner{
		keywords: keywords,

		hitsOnly:    true,
		hitContext:  20,
		baseDir:     os.TempDir(),
		parallelism: runtime.NumCPU(),
	}

	for _, o := range opts {
		if err := o(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Scanner) ScanFile(ctx context.Context, ifile string, scanResults chan<- output.ScanResult, errChan chan<- error) error {
	//
	// Recursively unarchive the files to be scanned
	//
	unarchiveResults := make(chan archive.UnarchiveResult)
	go func() {
		archive.UnarchiveRecursive(ctx, ifile, ".goscan-unar", unarchiveResults)
		close(unarchiveResults)
	}()

	//
	// Scan unarchived files for hits
	//
	var scanWg sync.WaitGroup
	scanWg.Add(s.parallelism)
	for i := 0; i < s.parallelism; i++ {
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
					hits, err := s.keywords.MatchFile(ur.File, s.hitContext)
					if err != nil {
						errChan <- err
						return
					}
					scanResults <- output.ScanResult{
						File: strings.Replace(strings.Replace(ur.File, path.Dir(ifile), "", -1), ".goscan-unar", "", -1),
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

	return nil
}
