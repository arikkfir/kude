package stream

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"github.com/arikkfir/kude/internal"
	"github.com/arikkfir/kude/internal/stream/generate"
	. "github.com/arikkfir/kude/internal/stream/processing"
	. "github.com/arikkfir/kude/internal/stream/sink"
	. "github.com/arikkfir/kude/internal/stream/types"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"
	"gopkg.in/yaml.v3"
	"strconv"
	"strings"
	"testing"
)

func TestStream2(t *testing.T) {
	yamlString := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: t1
  namespace: ns1
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: t1
  namespace: ns2
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    labeled: yes
  name: t2
  namespace: ns1
spec:
  selector:
    matchLabels:
      labeled: yes
  template:
    metadata:
      labels:
        labeled: yes
    spec:
      containers:
        - image: nginx
          name: nginx
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    labeled: yes
  name: t2
  namespace: ns2
spec:
  selector:
    matchLabels:
      labeled: yes
  template:
    metadata:
      labels:
        labeled: yes
    spec:
      containers:
        - image: nginx
          name: nginx
`
	w2 := &bytes.Buffer{}
	s := NewStream().
		Generate(generate.FromReader(strings.NewReader(yamlString))).
		Process(
			Tee(
				K8sTargetingFilter(nil, nil),
				NodeTransformerOf(AnnotateK8sResource("foo", "bar")),
			),
		).
		Sink(ToWriter(w2))
	if err := s.Execute(context.Background()); err != nil {
		t.Error(fmt.Errorf("failed executing stream: %w", err))
	}

	expectedYAMLString := `apiVersion: v1
kind: ServiceAccount
metadata:
  annotations:
    foo: bar
  name: t1
  namespace: ns1
---
apiVersion: v1
kind: ServiceAccount
metadata:
  annotations:
    foo: bar
  name: t1
  namespace: ns2
---
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    foo: bar
  labels:
    labeled: yes
  name: t2
  namespace: ns1
spec:
  selector:
    matchLabels:
      labeled: yes
  template:
    metadata:
      labels:
        labeled: yes
    spec:
      containers:
        - image: nginx
          name: nginx
---
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    foo: bar
  labels:
    labeled: yes
  name: t2
  namespace: ns2
spec:
  selector:
    matchLabels:
      labeled: yes
  template:
    metadata:
      labels:
        labeled: yes
    spec:
      containers:
        - image: nginx
          name: nginx`
	if actual, err := internal.FormatYAML(w2); err != nil {
		t.Fatalf("Failed to format YAML output: %s", err)
	} else if expected, err := internal.FormatYAML(strings.NewReader(expectedYAMLString)); err != nil {
		t.Errorf("Failed to format expected YAML output: %s", err)
	} else if strings.TrimSuffix(expected, "\n") != strings.TrimSuffix(actual, "\n") {
		edits := myers.ComputeEdits(span.URIFromPath("expected"), expected, actual)
		diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", expected, edits))
		t.Errorf("Incorrect output:\n===\n%s\n===", diff)
	}
}

func TestStream(t *testing.T) {
	const nodeCount = 1_000_000

	// Target channel for nodes to be sent to, and YAML paths for validation
	c := make(chan *yaml.Node, 1_000_000)
	include1Path, err := yamlpath.NewPath("$.metadata.include1")
	if err != nil {
		t.Fatalf("Failed to create include1 path: %v", err)
	}
	include2Path, err := yamlpath.NewPath("$.metadata.include2")
	if err != nil {
		t.Fatalf("Failed to create include2 path: %v", err)
	}
	fooAnnPath, err := yamlpath.NewPath("$.metadata.annotations.foo")
	if err != nil {
		t.Fatalf("Failed to create foo annotation path: %v", err)
	}

	// Start a validation thread to verify only selected nodes were sent
	done := make(chan int)
	go func() {
		count := 0
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Panic in validation goroutine: %v", r)
				done <- -1
			} else {
				done <- count
			}
		}()
		errors := 0
		for {
			node, ok := <-c
			if !ok {
				return
			} else if matches, err := include1Path.Find(node); err != nil {
				t.Errorf("Failed to inspect for include1 path: %v", err)
				errors++
			} else if len(matches) != 1 {
				t.Errorf("Expected 1 match of 'include1' property, got %d", len(matches))
				errors++
			} else if n := matches[0]; n.Kind != yaml.ScalarNode {
				t.Errorf("Expected scalar match of 'include1' property, got %d", n.Kind)
				errors++
			} else if n.Value != "true" {
				t.Errorf("Node with 'include1!=true' found!")
				errors++
			} else if matches, err := include2Path.Find(node); err != nil {
				t.Errorf("Failed to inspect for include2 path: %v", err)
				errors++
			} else if len(matches) != 1 {
				t.Errorf("Expected 1 match of 'include2' property, got %d", len(matches))
				errors++
			} else if n := matches[0]; n.Kind != yaml.ScalarNode {
				t.Errorf("Expected scalar match of 'include2' property, got %d", n.Kind)
				errors++
			} else if n.Value != "true" {
				t.Errorf("Node with 'include2!=true' found!")
				errors++
			} else if matches, err := fooAnnPath.Find(node); err != nil {
				t.Errorf("Failed to inspect for foo annotation path: %v", err)
				errors++
			} else if len(matches) != 1 {
				t.Errorf("Expected 1 match of 'foo' annotation, got %d", len(matches))
				errors++
			} else if n := matches[0]; n.Kind != yaml.ScalarNode {
				t.Errorf("Expected scalar match of 'foo' annotation, got %d", n.Kind)
				errors++
			} else if n.Value != "bar" {
				t.Errorf("Node with 'annotations.foo!=bar' found!")
				errors++
			} else {
				count++
			}
			if errors >= 10 {
				t.Logf("Stopped after %d errors", errors)
				break
			}
		}
	}()

	s := NewStream().
		Generate(func(ctx context.Context, target chan *yaml.Node) error {
			for i := 0; i < nodeCount; i++ {
				include1 := i%2 == 0
				include2 := i%3 == 0
				yml := `apiVersion: v1
kind: ServiceAccount
metadata:
  include1: ` + strconv.FormatBool(include1) + `
  include2: ` + strconv.FormatBool(include2) + `
  namespace: ns1
  name: sa` + strconv.Itoa(i)
				doc := yaml.Node{}
				if err := yaml.Unmarshal([]byte(yml), &doc); err != nil {
					return fmt.Errorf("failed unmarshalling YAML into node: %w", err)
				}
				target <- &doc
			}
			return nil
		}).
		Transform(YAMLPathFilter("$[?(@.metadata.include1==true)]")).
		Transform(YAMLPathFilter("$[?(@.metadata.include2==true)]")).
		Transform(NodeTransformerOf(AnnotateK8sResource("foo", "bar"))).
		//Process(
		//	TeeNodeProcessor([]NodeTransformer{
		//		YAMLPathNodeTransformer("$[?(@.metadata.include1==true)]"),
		//		YAMLPathNodeTransformer("$[?(@.metadata.include2==true)]"),
		//		NodeTransformerOf(K8sAnnotateNodeProcessor("foo", "bar")),
		//	})).
		Sink(ToChannel(c))
	if err := s.Execute(context.Background()); err != nil {
		t.Errorf("failed executing pipeline: %v", err)
	}
	close(c)

	// Wait for validation to finish
	count := <-done
	if count != 166667 {
		t.Errorf("Expected 166667 nodes, got %d", count)
	}
}
