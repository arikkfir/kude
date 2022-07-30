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
    path: $.metadata.annotations.unknown
    type:
      group: ""
      version: v1
      kind: ConfigMap
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

func TestCatalogResolveWithResourceWithoutAPIVersion(t *testing.T) {
	catalogYAML := `####
- group: apps
  version: v1
  kind: Deployment
  field:
    path: $.metadata.annotations.foo
    type:
      group: ""
      version: v1
      kind: ConfigMap`
	c := &catalog{}
	if err := c.loadFrom(strings.NewReader(catalogYAML)); err != nil {
		t.Fatalf("failed loading catalog: %v", err)
	}

	n := &yaml.Node{}
	inputYAML := `####
kind: Deployment
metadata:
  annotations:
    foo: bar
  name: d1
  namespace: ns`
	if err := yaml.Unmarshal([]byte(inputYAML), n); err != nil {
		t.Fatalf("failed decoding input YAML: %v", err)
	} else if err := c.resolve(&kyaml.RNode{N: n}, nil); err == nil {
		t.Fatalf("expected resolve to fail as resource has no 'apiVersion' property")
	} else if err.Error() != "failed to get API group and version for resource: apiVersion is missing" {
		t.Fatalf("unexpected different error message: %s", err)
	}
}

func TestCatalogResolveWithResourceWithoutKind(t *testing.T) {
	catalogYAML := `####
- group: apps
  version: v1
  kind: Deployment
  field:
    path: $.metadata.annotations.foo
    type:
      group: ""
      version: v1
      kind: ConfigMap`
	c := &catalog{}
	if err := c.loadFrom(strings.NewReader(catalogYAML)); err != nil {
		t.Fatalf("failed loading catalog: %v", err)
	}

	n := &yaml.Node{}
	y1 := `####
apiVersion: apps/v1
metadata:
  annotations:
    foo: bar
  name: d1
  namespace: ns`
	if err := yaml.Unmarshal([]byte(y1), n); err != nil {
		t.Fatalf("failed decoding input YAML: %v", err)
	} else if err := c.resolve(&kyaml.RNode{N: n}, nil); err == nil {
		t.Fatalf("expected resolve to fail as resource has no 'kind' property")
	} else if err.Error() != "failed to get kind for resource: empty or no 'kind' property" {
		t.Fatalf("unexpected different error message: %s", err)
	}

	y2 := `####
apiVersion: apps/v1
kind:
  a: b
metadata:
  annotations:
    foo: bar
  name: d1
  namespace: ns`
	if err := yaml.Unmarshal([]byte(y2), n); err != nil {
		t.Fatalf("failed decoding input YAML: %v", err)
	} else if err := c.resolve(&kyaml.RNode{N: n}, nil); err == nil {
		t.Fatalf("expected resolve to fail as resource has invalid 'kind' property")
	} else if err.Error() != "failed to get kind for resource: expected value node kind to be 8, got 4" {
		t.Fatalf("unexpected different error message: %s", err)
	}
}

func TestInvalidPathInReferencesCatalog(t *testing.T) {
	catalogYAML := `####
- group: apps
  version: v1
  kind: Deployment
  field:
    path: $$.metadata.annotations.foo
    type:
      group: ""
      version: v1
      kind: ConfigMap`
	c := &catalog{}
	if err := c.loadFrom(strings.NewReader(catalogYAML)); err == nil {
		t.Fatalf("expected catalog loading to fail due to invalid YAML path, but it did not")
	} else if err.Error() != "failed compiling YAML path: invalid path syntax at position 1, following \"$\"" {
		t.Fatalf("unexpected catalog loading error message: %v", err)
	}
}

func TestInvalidYAMLInReferencesCatalog(t *testing.T) {
	catalogYAML := `####
- group: apps
    version: v1
  kind: Deployment
  field:
    path: $.metadata.annotations.foo
    type:
      group: ""
      version: v1
      kind: ConfigMap`
	c := &catalog{}
	if err := c.loadFrom(strings.NewReader(catalogYAML)); err == nil {
		t.Fatalf("expected catalog loading to fail due to invalid YAML path, but it did not")
	} else if err.Error() != "failed decoding references catalog: yaml: line 3: mapping values are not allowed in this context" {
		t.Fatalf("unexpected catalog loading error message: %v", err)
	}
}
