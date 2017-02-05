package unar

import (
	"errors"
	"os/exec"
	"strings"
)

func Run(file string, outputDir string) error {
	output, err := exec.Command("unar", "-o", outputDir, file).CombinedOutput()
	if err != nil {
		return errors.New(strings.TrimSpace(string(output)))
	}
	return nil
}
