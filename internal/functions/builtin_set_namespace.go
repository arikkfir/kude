package functions

import (
	"context"
	"fmt"
	"github.com/arikkfir/kude/internal/stream"
	. "github.com/arikkfir/kude/internal/stream/generate"
	. "github.com/arikkfir/kude/internal/stream/processing"
	. "github.com/arikkfir/kude/internal/stream/sink"
	. "github.com/arikkfir/kude/internal/stream/types"
	"io"
	"log"
)

type SetNamespace struct {
	Namespace string            `json:"namespace" yaml:"namespace"`
	Includes  []TargetingFilter `json:"includes" yaml:"includes"`
	Excludes  []TargetingFilter `json:"excludes" yaml:"excludes"`
}

func (f *SetNamespace) Invoke(_ *log.Logger, _, _, _ string, r io.Reader, w io.Writer) error {
	if f.Namespace == "" {
		return fmt.Errorf("the '%s' property is required for this function", "name")
	}

	s := stream.NewStream().
		Generate(FromReader(r)).
		Process(
			Tee(
				K8sTargetingFilter(f.Includes, f.Excludes),
				NodeTransformerOf(SetK8sResourceNamespace(f.Namespace)),
			),
		).
		Sink(ToWriter(w))
	if err := s.Execute(context.Background()); err != nil {
		return fmt.Errorf("failed executing stream: %w", err)
	}
	return nil
}
