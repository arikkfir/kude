package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/arikkfir/kude/internal"
	"github.com/arikkfir/kude/test"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"io"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"testing"
)

const failureMessage = `%s: %w
STDERR:
=======
%s
-------
STDOUT:
=======
%s
-------`

func TestImportRemoteGithubKudePkg(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(fmt.Errorf("error getting working directory: %w", err))
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if pipeline, err := internal.NewPipeline(log.New(stderr, "", 0), pwd, kio.ByteWriter{Writer: stdout}); err != nil {
		t.Fatal(fmt.Errorf(failureMessage, "failed to build pipeline", err, stderr, stdout))
	} else if err := pipeline.Execute(); err != nil {
		t.Fatal(fmt.Errorf(failureMessage, "failed to execute pipeline", err, stderr, stdout))
	}
	defer os.RemoveAll(filepath.Join(pwd, ".kude"))

	formattedYAML, err := test.FormatYAML(stdout)
	if err != nil {
		t.Fatal(fmt.Errorf(failureMessage, "failed to format YAML output", err, stderr, stdout))
	}

	expectedPath := filepath.Join(pwd, "expected.yaml")
	expectedFile, err := os.Open(expectedPath)
	if err != nil {
		t.Fatal(fmt.Errorf("failed opening expected YAML file at '%s': %w", expectedPath, err))
	}
	expected, err := io.ReadAll(expectedFile)
	if err != nil {
		t.Fatal(fmt.Errorf("failed reading expected YAML file at '%s': %w", expectedPath, err))
	}
	if string(expected) != formattedYAML {
		edits := myers.ComputeEdits(span.URIFromPath("expected"), string(expected), formattedYAML)
		diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", string(expected), edits))
		t.Errorf("Incorrect output:\n===\n%s\n===", diff)
	}
}
