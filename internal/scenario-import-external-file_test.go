package internal

import (
	_ "embed"
	
	"github.com/arikkfir/kude/pkg"
	"github.com/arikkfir/kude/test"
	"io"
	"log"
	"testing"
)

//go:embed scenario-import-external-file.yaml
var scenarioImportExternalFileYAML string

func TestImportExternalFileDocker(t *testing.T) {
    factory := func(logger *log.Logger, pwd string, manifestReader io.Reader, output io.Writer) (pkg.Package, error) {
        return NewPackage(logger, pwd, manifestReader, output, false)
    }
    if err := test.RunScenario(t, "import-external-file", scenarioImportExternalFileYAML, factory); err != nil {
        t.Fatalf("Scenario failed: %v", err)
    }
}

func TestImportExternalFileInternal(t *testing.T) {
    factory := func(logger *log.Logger, pwd string, manifestReader io.Reader, output io.Writer) (pkg.Package, error) {
        return NewPackage(logger, pwd, manifestReader, output, true)
    }
    if err := test.RunScenario(t, "import-external-file", scenarioImportExternalFileYAML, factory); err != nil {
        t.Fatalf("Scenario failed: %v", err)
    }
}
