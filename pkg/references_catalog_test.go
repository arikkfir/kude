package kude

import (
	kyaml "github.com/arikkfir/kyaml/pkg"
	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func TestReferencesCatalog(t *testing.T) {
	catalogYAML := `####
- group: apps
  version: v1
  kind: Deployment
  field:
    path: $.metadata.annotations.foo
    type:
      group: ""
      version: v1
      kind: ConfigMap
- group: apps
  version: v1
  kind: Deployment
  field:
    path: $.metadata.annotations.parent
    type:
      group: apps
      version: v1
      kind: Deployment`
	c := &catalog{}
	if err := c.loadFrom(strings.NewReader(catalogYAML)); err != nil {
		t.Fatalf("failed loading catalog: %v", err)
	}

	n := &yaml.Node{}
	inputYAML := `####
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    foo: bar
    parent: d2
  name: d1
  namespace: ns`
	if err := yaml.Unmarshal([]byte(inputYAML), n); err != nil {
		t.Fatalf("failed decoding input YAML: %v", err)
	}

	rn := &kyaml.RNode{N: n}
	renamed := map[string]string{
		"apps/v1/Deployment/ns/d2": "d2-123",
		"v1/ConfigMap/ns/bar":      "bar-123",
	}
	if err := c.resolve(rn, renamed); err != nil {
		t.Fatalf("failed resolving references: %v", err)
	} else if foo, err := rn.GetAnnotation("foo"); err != nil {
		t.Fatalf("failed getting 'foo' annotation: %v", err)
	} else if foo != "bar-123" {
		t.Fatalf("unexpected 'foo' annotation value: %s", foo)
	} else if parent, err := rn.GetAnnotation("parent"); err != nil {
		t.Fatalf("failed getting 'parent' annotation: %v", err)
	} else if parent != "d2-123" {
		t.Fatalf("unexpected 'parent' annotation value: %s", parent)
	}

	delete(renamed, "v1/ConfigMap/ns/bar")
	renamed["v1/ConfigMap/ns2/bar"] = "bar-456"
	if err := c.resolve(rn, renamed); err != nil {
		t.Fatalf("failed resolving references: %v", err)
	} else if foo, err := rn.GetAnnotation("foo"); err != nil {
		t.Fatalf("failed getting 'foo' annotation: %v", err)
	} else if foo != "bar-123" { // stays bar-123 because deployment is not in namespace "ns2"
		t.Fatalf("unexpected annotation 'foo' value: %s", foo)
	}
}
