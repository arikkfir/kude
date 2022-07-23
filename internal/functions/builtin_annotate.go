package functions

import (
	"context"
	"fmt"
	"github.com/arikkfir/gstream/pkg"
	. "github.com/arikkfir/gstream/pkg/generate"
	. "github.com/arikkfir/gstream/pkg/processing"
	. "github.com/arikkfir/gstream/pkg/sink"
	. "github.com/arikkfir/gstream/pkg/types"
	"io"
	"log"
	"os"
	"path/filepath"
)

type Annotate struct {
	Name     string            `json:"name" yaml:"name"`
	Value    string            `json:"value" yaml:"value"`
	Path     string            `json:"path" yaml:"path"`
	Includes []TargetingFilter `json:"includes" yaml:"includes"`
	Excludes []TargetingFilter `json:"excludes" yaml:"excludes"`
}

func (f *Annotate) Invoke(_ *log.Logger, pwd, _, _ string, r io.Reader, w io.Writer) error {
	if f.Name == "" {
		return fmt.Errorf("the '%s' property is required for this function", "name")
	}

	var value string
	if f.Path != "" {
		path := f.Path
		if !filepath.IsAbs(f.Path) {
			path = filepath.Join(pwd, f.Path)
		}
		valueFileBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed reading '%s': %w", path, err)
		}
		value = string(valueFileBytes)
	} else {
		value = f.Value
	}

	s := stream.NewStream().
		Generate(FromReader(r)).
		Process(
			Tee(
				K8sTargetingFilter(f.Includes, f.Excludes),
				NodeTransformerOf(AnnotateK8sResource(f.Name, value)),
			),
		).
		Sink(ToWriter(w))
	if err := s.Execute(context.Background()); err != nil {
		return fmt.Errorf("failed executing stream: %w", err)
	}
	return nil
}
