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
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"log"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

const (
	testBigResourcesCount = 10_000
)

//go:embed testdata/big_input_manifest.yaml
var testBigResourcesManifest string

//go:embed testdata/big_input_deployment.yaml
var testBigResourcesDeploymentPattern string

func TestBigResourcesInput(t *testing.T) {
	enPrinter := message.NewPrinter(language.English)
	const httpPath = "/metrics"
	const httpPort = ":9000"
	http.Handle(httpPath, promhttp.Handler())
	go func() {
		t.Logf("Starting Prometheus HTTP server listening on %s%s", httpPath, httpPort)
		if err := http.ListenAndServe(httpPort, nil); err != nil {
			t.Logf("HTTP server failed: %v", err)
		}
	}()

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
	t.Logf("Test temp directory placed at: %s", dir)

	// Write the manifest
	if err := ioutil.WriteFile(filepath.Join(dir, "kude.yaml"), []byte(testBigResourcesManifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Build actual YAML buffer; we will also collect and save all generated nodes in preparation of generating expected YAML
	t.Logf("Generating input resources YAML from %s resources", enPrinter.Sprint(testBigResourcesCount))
	actualBuffer := &bytes.Buffer{}
	actualEncoder := yaml.NewEncoder(actualBuffer)
	expectedNodes := make([]*yaml.Node, 0, testBigResourcesCount)
	for i := 0; i < testBigResourcesCount; i++ {
		yamlString := strings.ReplaceAll(testBigResourcesDeploymentPattern, "$$$", strconv.Itoa(i))
		node := &yaml.Node{}
		if err := yaml.Unmarshal([]byte(yamlString), node); err != nil {
			t.Fatalf("failed to build test YAML: %v", err)
		}
		node = node.Content[0] // Get the underlying MappingNode from the DocumentNode
		if err := actualEncoder.Encode(node); err != nil {
			t.Fatalf("failed to encode test input YAML: %v", err)
		}

		if err := internal.SetAnnotation(node, "foo1", "bar1"); err != nil {
			t.Fatalf("failed to set annotation foo1: %v", err)
		} else if err := internal.SetAnnotation(node, "foo2", "bar2"); err != nil {
			t.Fatalf("failed to set annotation foo2: %v", err)
		} else {
			expectedNodes = append(expectedNodes, node)
		}
	}
	actualEncoder.Close()

	// Build expected YAML buffer
	t.Logf("Generating expected output resources YAML")
	sort.Sort(ByType(expectedNodes))
	expectedBuffer := &bytes.Buffer{}
	expectedEncoder := yaml.NewEncoder(expectedBuffer)
	for _, node := range expectedNodes {
		if err := expectedEncoder.Encode(node); err != nil {
			t.Fatalf("failed to encode expected test YAML: %v", err)
		}
	}
	expectedEncoder.Close()

	// Write the resources.yaml file
	if err := ioutil.WriteFile(filepath.Join(dir, "resources.yaml"), actualBuffer.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write resources.yaml: %v", err)
	}

	// Run the pipeline
	t.Logf("Creating pipeline execution")
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

	t.Logf("Test finished. Will wait for 15sec to allow metrics to be scraped and sent")
	time.Sleep(15 * time.Second)
}
