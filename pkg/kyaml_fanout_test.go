package pkg

import (
	_ "embed"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
	"testing"
)

//go:embed testdata/deployment.yaml
var deployment string

func TestFanout(t *testing.T) {
	resources := make([]*yaml.RNode, 0)
	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{&kio.ByteReader{Reader: strings.NewReader(deployment)}},
		Filters: []kio.Filter{
			Fanout(
				yaml.FilterFunc(func(r *yaml.RNode) (*yaml.RNode, error) {
					if err := r.SetAnnotations(map[string]string{"foo": "bar"}); err != nil {
						t.Errorf("unexpected error: %v", err)
					}
					return r, nil
				}),
			),
		},
		Outputs: []kio.Writer{kio.WriterFunc(func(rns []*yaml.RNode) error {
			resources = append(resources, rns...)
			return nil
		})},
	}
	if err := pipeline.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if len(resources) != 1 {
		t.Fatalf("unexpected number of resources: %d (expected 1)", len(resources))
	} else if fooValue := resources[0].GetAnnotations()["foo"]; fooValue != "bar" {
		t.Errorf("expected annotation 'foo' is missing or incorrect; foo=%s, found only: %v", fooValue, resources[0].GetAnnotations())
	}
}
