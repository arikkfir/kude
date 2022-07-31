package kude

import (
	"github.com/arikkfir/kyaml/pkg"
	"gopkg.in/yaml.v3"
	"testing"
)

func TestGetResourcePreviousName(t *testing.T) {
	var yml string

	n := &yaml.Node{}
	if err := yaml.Unmarshal([]byte(`string`), n); err != nil {
		t.Errorf("failed decoding input YAML: %v", err)
	} else if previousName, err := GetResourcePreviousName(&kyaml.RNode{N: n}); err == nil {
		t.Errorf("expected non-mapping document to fail, but it did not; got name: '%s'", previousName)
	}

	yml = `####
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d1-123
  namespace: ns`
	if err := yaml.Unmarshal([]byte(yml), n); err != nil {
		t.Errorf("failed decoding input YAML: %v", err)
	} else if previousName, err := GetResourcePreviousName(&kyaml.RNode{N: n}); err != nil {
		t.Errorf("failed getting previous name: %v", err)
	} else if previousName != "" {
		t.Errorf("expected empty previous name, but got '%s'", previousName)
	}

	r := &kyaml.RNode{N: n}
	if err := r.SetAnnotation(PreviousNameAnnotationName, "d1"); err != nil {
		t.Errorf("failed setting annotation: %v", err)
	} else if previousName, err := GetResourcePreviousName(r); err != nil {
		t.Errorf("failed setting annotation: %v", err)
	} else if previousName != "d1" {
		t.Errorf("GetResourcePreviousName(r) != \"d1\"")
	}
}
