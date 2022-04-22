package test

import (
	"bytes"
	"fmt"
	"github.com/arikkfir/kude/internal"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"strings"
	"testing"
)

func InvokePipelineForTest(t *testing.T, path string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(fmt.Errorf("error creating absolute path from '%s': %w", path, err))
	}

	stdout, _ := Capture(true, false, func() {
		pipeline, err := internal.NewPipeline(log.New(os.Stderr, "> ", 0), absPath, kio.ByteWriter{Writer: os.Stdout})
		if err != nil {
			t.Fatal(fmt.Errorf("failed to build pipeline from '%s': %w", path, err))
		}

		if err := pipeline.Execute(); err != nil {
			t.Fatal(fmt.Errorf("pipeline execution at '%s' failed: %w", path, err))
		}
	})

	actualFormatted := bytes.Buffer{}
	decoder := yaml.NewDecoder(strings.NewReader(stdout))
	encoder := yaml.NewEncoder(&actualFormatted)
	encoder.SetIndent(2)
	for {
		var data interface{}
		if err := decoder.Decode(&data); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(fmt.Errorf("failed decoding YAML: %w\n======\n%s\n========", err, stdout))
		}
		if err := encoder.Encode(data); err != nil {
			t.Fatal(fmt.Errorf("failed encoding struct: %w", err))
		}
	}

	expectedPath := filepath.Join(absPath, "expected.yaml")
	expectedFile, err := os.Open(expectedPath)
	if err != nil {
		t.Fatal(fmt.Errorf("failed opening expected YAML file at '%s': %w", expectedPath, err))
	}
	expected, err := io.ReadAll(expectedFile)
	if err != nil {
		t.Fatal(fmt.Errorf("failed reading expected YAML file at '%s': %w", expectedPath, err))
	}
	if string(expected) != actualFormatted.String() {
		edits := myers.ComputeEdits(span.URIFromPath("expected"), string(expected), actualFormatted.String())
		diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", string(expected), edits))
		t.Errorf("Incorrect output:\n===\n%s\n===", diff)
	}
}
