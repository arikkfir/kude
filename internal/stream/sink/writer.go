package sink

import (
	"context"
	"fmt"
	. "github.com/arikkfir/kude/internal/stream/types"
	"gopkg.in/yaml.v3"
	"io"
)

type writerNodeSink struct {
	encoder *yaml.Encoder
}

func (s *writerNodeSink) Close() error {
	return s.encoder.Close()
}

func (s *writerNodeSink) Process(_ context.Context, node *yaml.Node) error {
	if err := s.encoder.Encode(node); err != nil {
		return fmt.Errorf("failed encoding node: %w", err)
	}
	return nil
}

func ToWriter(w io.Writer) NodeSink {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return &writerNodeSink{encoder}
}
