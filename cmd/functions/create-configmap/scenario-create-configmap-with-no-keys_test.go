package main

import (
	_ "embed"
	"github.com/arikkfir/kude/internal"
	"github.com/arikkfir/kude/pkg"
	"github.com/arikkfir/kude/test"
	"io"
	"log"
	"testing"
)

//go:embed scenario-create-configmap-with-no-keys.yaml
var scenarioCreateConfigmapWithNoKeysYAML string

func TestCreateConfigmapWithNoKeysDocker(t *testing.T) {
    factory := func(logger *log.Logger, pwd string, manifestReader io.Reader, output io.Writer) (pkg.Package, error) {
        return internal.NewPackage(logger, pwd, manifestReader, output, false)
    }
    if err := test.RunScenario(t, "create-configmap-with-no-keys", scenarioCreateConfigmapWithNoKeysYAML, factory); err != nil {
        t.Fatalf("Scenario failed: %v", err)
    }
}

func TestCreateConfigmapWithNoKeysInternal(t *testing.T) {
    factory := func(logger *log.Logger, pwd string, manifestReader io.Reader, output io.Writer) (pkg.Package, error) {
        return internal.NewPackage(logger, pwd, manifestReader, output, true)
    }
    if err := test.RunScenario(t, "create-configmap-with-no-keys", scenarioCreateConfigmapWithNoKeysYAML, factory); err != nil {
        t.Fatalf("Scenario failed: %v", err)
    }
}
