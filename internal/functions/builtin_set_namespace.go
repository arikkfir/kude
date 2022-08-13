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
	Namespace string                  `mapstructure:"namespace"`
	Includes  []kyaml.TargetingFilter `mapstructure:"includes"`
	Excludes  []kyaml.TargetingFilter `mapstructure:"excludes"`
}

func (f *SetNamespace) Invoke(_ *log.Logger, _, _, _ string, r io.Reader, w io.Writer) error {
	if f.Namespace == "" {
		return fmt.Errorf("the '%s' property is required for this function", "name")
	}
	f.Excludes = append(f.Excludes, kyaml.TargetingFilter{APIVersion: "v1", Kind: "Namespace"})
	f.Excludes = append(f.Excludes, kyaml.TargetingFilter{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole"})
	f.Excludes = append(f.Excludes, kyaml.TargetingFilter{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRoleBinding"})
	f.Excludes = append(f.Excludes, kyaml.TargetingFilter{APIVersion: "admissionregistration.k8s.io/v1", Kind: "ValidatingWebhookConfiguration"})
	f.Excludes = append(f.Excludes, kyaml.TargetingFilter{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition"})

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
