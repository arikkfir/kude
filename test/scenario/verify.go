package scenario

import (
	"bytes"
	"fmt"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"io"
	"regexp"
	"strings"
	"testing"
)

type PackageRunner func(scenario *Scenario, stdout, stderr io.Writer) error

func (s *Scenario) VerifyError(t *testing.T, err error) {
	if s.ExpectedError != "" {
		if err == nil {
			t.Fatalf("Error expected but none received; expected: %s", s.ExpectedError)
		} else if match, matchErr := regexp.Match(s.ExpectedError, []byte(err.Error())); matchErr != nil {
			t.Fatalf("Failed to compare expected error: %s", matchErr)
		} else if match {
			// as expected
		} else {
			t.Fatalf("Incorrect error received during package creation! expected '%s', received: %s", s.ExpectedError, err)
		}
	} else if err != nil {
		t.Fatalf("Unexpected error during package execution: %s", err)
	}
}

func (s *Scenario) VerifyStdout(t *testing.T, stdout *bytes.Buffer) {
	actual, err := s.formatYAML(stdout)
	if err != nil {
		t.Fatalf("Failed to format YAML output: %s", err)
	}

	if s.ExpectedContents != "" {
		if expected, err := s.formatYAML(strings.NewReader(s.ExpectedContents)); err != nil {
			t.Fatalf("Failed to format expected YAML output: %s", err)
		} else if strings.TrimSuffix(expected, "\n") != strings.TrimSuffix(actual, "\n") {
			edits := myers.ComputeEdits(span.URIFromPath("expected"), expected, actual)
			diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", expected, edits))
			t.Fatalf("Incorrect output:\n===\n%s\n===", diff)
		}
	} else {
		t.Fatalf("scenario contains neither the 'expected' nor the 'expectedError' property")
	}
}
