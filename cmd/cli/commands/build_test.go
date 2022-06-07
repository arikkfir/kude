package commands

import (
	"bytes"
	_ "embed"
	"errors"
	"github.com/arikkfir/kude/test/scenario"
	"github.com/arikkfir/kude/test/util"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//go:embed testdata/simple-scenario.yaml
var simpleScenarioYAML string

func TestBuildPathIsNonExisting(t *testing.T) {
	b := builder{
		path:   "/non/existing/path",
		logger: log.New(&util.TestWriter{T: t}, "", 0),
		stdout: &util.TestWriter{T: t},
	}
	if err := b.Invoke(); err == nil {
		t.Errorf("Command should have failed")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Command should have failed with error %s, got: %s", os.ErrNotExist, err)
	}
}

func TestBuildPathIsDirWithoutKudeYAMLFile(t *testing.T) {
	dir, err := os.MkdirTemp("", t.Name())
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	defer func() {
		os.RemoveAll(dir)
	}()

	b := builder{
		path:   dir,
		logger: log.New(&util.TestWriter{T: t}, "", 0),
		stdout: &util.TestWriter{T: t},
	}
	if err := b.Invoke(); err == nil {
		t.Errorf("Command should have failed")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Command should have failed with error %s, got: %s", os.ErrNotExist, err)
	}
}

func TestBuildPathIsDirWithKudeYAMLFileIsDir(t *testing.T) {
	dir, err := os.MkdirTemp("", t.Name())
	kudeYAMLFile := filepath.Join(dir, "kude.yaml")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	} else if err := os.Mkdir(kudeYAMLFile, 0644); err != nil {
		t.Fatalf("failed to create temporary kude.yaml dir: %s", err)
	}
	defer func() {
		os.RemoveAll(dir)
	}()

	b := builder{
		path:   dir,
		logger: log.New(&util.TestWriter{T: t}, "", 0),
		stdout: &util.TestWriter{T: t},
	}
	if err := b.Invoke(); err == nil {
		t.Fatalf("Command should have failed")
	} else if err.Error() != "path '"+kudeYAMLFile+"' is a directory, expected a file" {
		t.Fatalf("Command failed with incorrect error: %s", err)
	}
}

func TestBuildPathIsDirWithKudeYAMLFile(t *testing.T) {
	s, err := scenario.OpenScenario("TestBuildPathIsDirWithKudeYAMLFile", strings.NewReader(simpleScenarioYAML))
	if err != nil {
		t.Fatalf("failed to open scenario: %s", err)
	}

	stdout := bytes.Buffer{}
	b := builder{
		path:   s.Dir,
		logger: log.New(&util.TestWriter{T: t}, "", 0),
		stdout: &stdout,
	}
	if err := b.Invoke(); err != nil {
		s.VerifyError(t, err)
	} else {
		s.VerifyStdout(t, &stdout)
	}
}

func TestBuildPathIsKudeYAMLFile(t *testing.T) {
	s, err := scenario.OpenScenario("TestBuildPathIsKudeYAMLFile", strings.NewReader(simpleScenarioYAML))
	if err != nil {
		t.Fatalf("failed to open scenario: %s", err)
	}

	stdout := bytes.Buffer{}
	b := builder{
		path:   s.ManifestPath,
		logger: log.New(&util.TestWriter{T: t}, "", 0),
		stdout: &stdout,
	}
	if err := b.Invoke(); err != nil {
		s.VerifyError(t, err)
	} else {
		s.VerifyStdout(t, &stdout)
	}
}
