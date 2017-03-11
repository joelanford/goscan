package output

import (
	"io"

	yaml "gopkg.in/yaml.v1"
)

type YAMLSummaryWriter struct {
	writer io.Writer
}

func NewYAMLSummaryWriter(writer io.Writer) *YAMLSummaryWriter {
	return &YAMLSummaryWriter{
		writer: writer,
	}
}

func (w *YAMLSummaryWriter) WriteSummary(sum ScanSummary) error {
	data, err := yaml.Marshal(&sum)
	if err != nil {
		return err
	}
	_, err = w.writer.Write(data)
	return err
}
