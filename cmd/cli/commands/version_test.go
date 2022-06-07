package commands

import (
	"bytes"
	"github.com/arikkfir/kude/pkg"
	"log"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	stderr := bytes.Buffer{}
	v := versioner{logger: log.New(&stderr, "", 0)}
	if err := v.Invoke(); err != nil {
		t.Errorf("Command failed: %s", err)
	} else if strings.TrimSpace(stderr.String()) != pkg.GetVersion().String() {
		t.Errorf("Expected output '%s', got '%s'", pkg.GetVersion().String(), stderr.String())
	}
}
