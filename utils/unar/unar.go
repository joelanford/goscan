package unar

import (
	"errors"
	"os/exec"
)

func Run(file string, outputDir string) error {
	output, err := exec.Command("unar", "-o", outputDir, file).CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	return nil
}
