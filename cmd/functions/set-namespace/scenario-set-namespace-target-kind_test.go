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

//go:embed scenario-set-namespace-target-kind.yaml
var scenarioSetNamespaceTargetKindYAML string

func TestSetNamespaceTargetKindDocker(t *testing.T) {
    factory := func(logger *log.Logger, pwd string, manifestReader io.Reader, output io.Writer) (pkg.Package, error) {
        return internal.NewPackage(logger, pwd, manifestReader, output, false)
    }
    if err := test.RunScenario(t, "set-namespace-target-kind", scenarioSetNamespaceTargetKindYAML, factory); err != nil {
        t.Fatalf("Scenario failed: %v", err)
    }
}

func TestSetNamespaceTargetKindInternal(t *testing.T) {
    factory := func(logger *log.Logger, pwd string, manifestReader io.Reader, output io.Writer) (pkg.Package, error) {
        return internal.NewPackage(logger, pwd, manifestReader, output, true)
    }
    if err := test.RunScenario(t, "set-namespace-target-kind", scenarioSetNamespaceTargetKindYAML, factory); err != nil {
        t.Fatalf("Scenario failed: %v", err)
    }
}
