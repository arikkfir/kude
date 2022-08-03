package functions

import (
	"context"
	"fmt"
	"github.com/arikkfir/gstream/pkg"
	. "github.com/arikkfir/gstream/pkg/generate"
	. "github.com/arikkfir/gstream/pkg/processing"
	. "github.com/arikkfir/gstream/pkg/sink"
	. "github.com/arikkfir/gstream/pkg/types"
	"github.com/arikkfir/kyaml/pkg"
	"github.com/arikkfir/kyaml/pkg/kstream"
	"io"
	"log"
)

type SetNamespace struct {
	Namespace string                  `json:"namespace" yaml:"namespace"`
	Includes  []kyaml.TargetingFilter `json:"includes" yaml:"includes"`
	Excludes  []kyaml.TargetingFilter `json:"excludes" yaml:"excludes"`
}

func (f *SetNamespace) Invoke(_ *log.Logger, _, _, _ string, r io.Reader, w io.Writer) error {
	if f.Namespace == "" {
		return fmt.Errorf("the '%s' property is required for this function", "name")
	}

	s := stream.NewStream().
		Generate(FromReader(r)).
		Process(
			Tee(
				kstream.FilterResource(f.Includes, f.Excludes),
				NodeTransformerOf(kstream.SetResourceNamespace(f.Namespace)),
			),
		).
		Sink(ToWriter(w))
	if err := s.Execute(context.Background()); err != nil {
		return fmt.Errorf("failed executing stream: %w", err)
	}
	return nil
}