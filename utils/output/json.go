package output

import "io"
import "encoding/json"

type JSONSummaryWriter struct {
	encoder *json.Encoder
	Prefix  string
	Indent  string
}

func NewJSONSummaryWriter(writer io.Writer, prefix, indent string) *JSONSummaryWriter {
	enc := json.NewEncoder(writer)
	enc.SetIndent(prefix, indent)
	return &JSONSummaryWriter{
		encoder: enc,
	}
}

func (w *JSONSummaryWriter) WriteSummary(sum ScanSummary) error {
	return w.encoder.Encode(&sum)
}
