package test

import (
	"fmt"
	"github.com/arikkfir/kude/test"
	"os"
	"testing"
)

func TestScenario(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(fmt.Errorf("error getting working directory: %w", err))
	}
	test.InvokePipelineForTest(t, pwd)
}
