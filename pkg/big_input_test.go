package kude

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"github.com/arikkfir/kude/internal"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"io/ioutil"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strconv"

	"log"
	"os"
	"strings"
	"testing"
)

const (
	testBigResourcesCount = 100_000
)

//go:embed testdata/big_input_manifest.yaml
var testBigResourcesManifest string

//go:embed testdata/big_input_deployment.yaml
var testBigResourcesDeploymentPattern string

func TestBigResourcesInput(t *testing.T) {
	// Create temporary directory for the pipeline
	dir, err := os.MkdirTemp("", t.Name())
	if err != nil {
		t.Fatalf("failed to create temporary directory: %v", err)
	}
	defer func() {
		if !t.Failed() {
			os.RemoveAll(dir)
		}
	}()

	// Write the manifest
	if err := ioutil.WriteFile(filepath.Join(dir, "kude.yaml"), []byte(testBigResourcesManifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Build YAML files (resources.yaml, and expected.yaml)
	actualBuffer := &bytes.Buffer{}
	expectedBuffer := &bytes.Buffer{}
	for i := 0; i < testBigResourcesCount; i++ {
		if i > 0 {
			if _, err := actualBuffer.WriteString("---\n"); err != nil {
				t.Fatalf("failed to write separator: %v", err)
			}
			if _, err := expectedBuffer.WriteString("---\n"); err != nil {
				t.Fatalf("failed to write separator: %v", err)
			}
		}

		yamlString := strings.ReplaceAll(testBigResourcesDeploymentPattern, "$$$", strconv.Itoa(i))
		node := yaml.MustParse(yamlString)
		if _, err := actualBuffer.WriteString(yamlString); err != nil {
			t.Fatalf("failed to write YAML to resources buffer: %v", err)
		}

		if err := node.SetAnnotations(map[string]string{"foo": "bar"}); err != nil {
			t.Fatalf("failed to set annotations: %v", err)
		} else if _, err := expectedBuffer.WriteString(node.MustString()); err != nil {
			t.Fatalf("failed to write YAML to expected buffer: %v", err)
		}
	}

	// Write the resources.yaml file
	if err := ioutil.WriteFile(filepath.Join(dir, "resources.yaml"), actualBuffer.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write resources.yaml: %v", err)
	}

	// Run the pipeline
	stdout := &bytes.Buffer{}
	if p, err := NewPipeline(dir); err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	} else if e, err := NewExecution(p, log.New(&internal.TestWriter{T: t}, "[pipeline] ", 0)); err != nil {
		t.Fatalf("failed to create pipeline execution: %v", err)
	} else if err := e.ExecuteToWriter(context.Background(), stdout); err != nil {
		t.Fatalf("failed to execute pipeline: %v", err)
	} else if expected, err := internal.FormatYAML(expectedBuffer); err != nil {
		t.Fatalf("failed to format expected YAML: %v", err)
	} else if actual, err := internal.FormatYAML(stdout); err != nil {
		t.Fatalf("failed to format actual YAML: %v", err)
	} else {
		_ = os.WriteFile(filepath.Join(dir, "actual.yaml"), []byte(actual), 0644)
		_ = os.WriteFile(filepath.Join(dir, "expected.yaml"), []byte(expected), 0644)
		if strings.TrimSuffix(expected, "\n") != strings.TrimSuffix(actual, "\n") {
			edits := myers.ComputeEdits(span.URIFromPath("expected"), expected, actual)
			diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", expected, edits))
			t.Fatalf("Incorrect output:\n===\n%s\n===", diff)
		}
	}
}
