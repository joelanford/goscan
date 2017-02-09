package archive

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	filetype "gopkg.in/h2non/filetype.v1"
)

func init() {
	rpmType := filetype.AddType("rpm", "application/x-rpm")
	filetype.AddMatcher(rpmType, func(header []byte) bool {
		return len(header) >= 4 && header[0] == 0xED && header[1] == 0xAB && header[2] == 0xEE && header[3] == 0xDB
	})
}

type UnarchiveResult struct {
	File  string
	Error error
}

func CanUnarchive(file string) (bool, error) {
	k, err := filetype.MatchFile(file)
	if err != nil {
		return false, err
	}

	canUnarchive := k.Extension == "zip" ||
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
		strings.HasSuffix(file, ".img")

	return canUnarchive, nil
}

func Unarchive(file string, outputDir string) error {
	output, err := exec.Command("unar", "-o", outputDir, file).CombinedOutput()
	if err != nil {
		return errors.New(strings.TrimSpace(string(output)))
	}
	return nil
}

func UnarchiveRecursive(ctx context.Context, file, extension string, results chan<- UnarchiveResult) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		if err := filepath.Walk(file, unarchiveWalk(ctx, &wg, extension, results)); err != nil {
			results <- UnarchiveResult{Error: err}
		}
		wg.Done()
	}()
	wg.Wait()
}

func unarchiveWalk(ctx context.Context, wg *sync.WaitGroup, extension string, results chan<- UnarchiveResult) filepath.WalkFunc {
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

		if ok, err := CanUnarchive(file); err != nil {
			return err
		} else if ok {
			unarchivePath := file + extension

			//
			// We can ignore most errors, because they're usually problems unarchiving.
			// Since we scan the archive file itself, it isn't a huge deal if we fail
			// to unarchive something.
			//
			// TODO: WE should create errors for issues that jeopardize the legitimacy
			//       of the scan. For example, a full disk where our scratch space is.
			//
			_ = Unarchive(file, unarchivePath)
			results <- UnarchiveResult{File: file}
			if _, err := os.Stat(unarchivePath); !os.IsNotExist(err) {
				wg.Add(1)
				go func() {
					if err := filepath.Walk(unarchivePath, unarchiveWalk(ctx, wg, extension, results)); err != nil {
						results <- UnarchiveResult{Error: err}
					}
					wg.Done()
				}()
			}
		} else {
			results <- UnarchiveResult{File: file}
		}
		return nil
	}
}
