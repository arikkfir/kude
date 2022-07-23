package functions

import (
	"context"
	"fmt"
	"github.com/arikkfir/kude/internal/stream"
	. "github.com/arikkfir/kude/internal/stream/generate"
	. "github.com/arikkfir/kude/internal/stream/sink"
	"io"
	"log"
	"strings"
)

type CreateNamespace struct {
	Name string `json:"name" yaml:"name"`
}

func (f *CreateNamespace) Invoke(_ *log.Logger, _, _, _ string, r io.Reader, w io.Writer) error {
	if f.Name == "" {
		return fmt.Errorf("%s is required for creating namespaces", "name")
	}

	namespace := `
apiVersion: v1
kind: Namespace
metadata:
  name: ` + f.Name + `
`
	s := stream.NewStream().
		Generate(FromReader(strings.NewReader(namespace))).
		Generate(FromReader(r)).
		Sink(ToWriter(w))
	if err := s.Execute(context.Background()); err != nil {
		return fmt.Errorf("pipeline invocation failed: %w", err)
	}
	return nil
}
