package kude

import (
	"bytes"
	"context"
	"embed"
	_ "embed"
	"fmt"
	"github.com/arikkfir/kude/internal"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	"log"
	"os"
	"strings"
	"testing"
)

// content holds our static web server content.
//go:embed testdata/scenario-*.yaml
var scenarios embed.FS

const (
	scenarioAPIVersion = "kude.kfirs.com/v1alpha1"
	scenarioKind       = "Scenario"
)

func TestScenarios(t *testing.T) {
	entries, err := scenarios.ReadDir("testdata")
	if err != nil {
		t.Fatalf("Failed reading scenarios: %v", err)
	}

	execute := func(t *testing.T, name string, inlineBuiltinFunctions bool) {
		var expectedContents, expectedError string

		b, err := scenarios.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatalf("failed to read scenario '%s': %v", name, err)
		}

		rn, err := yaml.Parse(string(b))
		if err != nil {
			t.Fatalf("failed to parse scenario at '%s': %v", name, err)
		}

		if rn.GetApiVersion() != scenarioAPIVersion {
			t.Fatalf("incorrect scenario API version at '%s'; expected '%s', got '%s'", name, scenarioAPIVersion, rn.GetApiVersion())
		} else if rn.GetKind() != scenarioKind {
			t.Fatalf("incorrect scenario kind at '%s'; expected '%s', got '%s'", name, scenarioKind, rn.GetKind())
		}

		dir, err := os.MkdirTemp("", name)
		if err != nil {
			t.Fatalf("failed to create tempdir: %v", err)
		} else if pipelineRN := rn.Field("pipeline"); pipelineRN == nil {
			t.Fatalf("Scenario does not have 'pipeline' property!")
		} else if err := yaml.WriteFile(pipelineRN.Value, filepath.Join(dir, "kude.yaml")); err != nil {
			t.Fatalf("failed to write pipeline manifest: %v", err)
		}

		resourcesRN := rn.Field("resources")
		if resourcesRN != nil {
			fields, err := resourcesRN.Value.Fields()
			if err != nil {
				t.Fatalf("failed to get scenario resources node: %v", err)
			}

			for _, field := range fields {
				fieldNode := resourcesRN.Value.Field(field)
				if fieldNode == nil {
					t.Fatalf("failed to get scenario resource node '%s'", field)
				}

				filename := fieldNode.Key.YNode().Value
				contents := fieldNode.Value.YNode().Value
				targetFile := filepath.Join(dir, filename)
				targetDir := filepath.Dir(targetFile)
				if err := os.MkdirAll(targetDir, 0755); err != nil {
					t.Errorf("failed creating directory '%s': %v", targetDir, err)
				} else if err := ioutil.WriteFile(targetFile, []byte(contents), 0644); err != nil {
					t.Errorf("failed writing resource '%s' to '%s': %v", filename, targetFile, err)
				}
			}
		}

		expectedField := rn.Field("expected")
		if expectedField != nil {
			expectedContents = expectedField.Value.YNode().Value
		}

		expectedErrorField := rn.Field("expectedError")
		if expectedErrorField != nil {
			expectedError = expectedErrorField.Value.YNode().Value
		}

		var p Pipeline
		if inlineBuiltinFunctions {
			if inliningPipeline, err := newInliningPipeline(dir); err != nil {
				t.Fatalf("failed to create inlining pipeline: %v", err)
			} else {
				p = inliningPipeline
			}
		} else {
			if pipeline, err := NewPipeline(dir); err != nil {
				t.Fatalf("failed to create pipeline: %v", err)
			} else {
				p = pipeline
			}
		}

		e, err := NewExecution(p, log.New(&internal.TestWriter{T: t}, "", 0))
		if err != nil {
			t.Fatalf("failed creating pipeline execution: %v", err)
		}

		stdout := bytes.Buffer{}
		if err := e.ExecuteToWriter(context.Background(), &stdout); err != nil {
			if expectedError != "" {
				if match, matchErr := regexp.Match(expectedError, []byte(err.Error())); matchErr != nil {
					t.Errorf("Failed to compare expected error: %s", matchErr)
				} else if match {
					// as expected
				} else {
					t.Errorf("Incorrect error received during package creation! expected '%s', received: %s", expectedError, err)
				}
			} else if err != nil {
				t.Errorf("Pipeline failed: %v", err)
			}
		} else {
			if expectedError != "" {
				t.Errorf("Error expected but none received; expected: %s", expectedError)
			}
		}

		if expectedContents != "" {
			actual, err := internal.FormatYAML(&stdout)
			if err != nil {
				t.Fatalf("Failed to format YAML output: %s", err)
			}
			if expected, err := internal.FormatYAML(strings.NewReader(expectedContents)); err != nil {
				t.Errorf("Failed to format expected YAML output: %s", err)
			} else if strings.TrimSuffix(expected, "\n") != strings.TrimSuffix(actual, "\n") {
				edits := myers.ComputeEdits(span.URIFromPath("expected"), expected, actual)
				diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", expected, edits))
				t.Errorf("Incorrect output:\n===\n%s\n===", diff)
			}
		}

		if err := os.RemoveAll(dir); err != nil {
			log.Printf("failed to remove temp dir '%s': %s", dir, err)
		}
	}

	for _, entry := range entries {
		name := entry.Name()
		t.Run(name+"@Docker", func(t *testing.T) { execute(t, name, false) })
		t.Run(name+"@Inline", func(t *testing.T) { execute(t, name, true) })
	}
}
