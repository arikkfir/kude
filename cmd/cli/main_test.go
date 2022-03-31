package main

import (
	"bytes"
	"fmt"
	"github.com/arikkfir/kude/cmd/cli/commands"
	"github.com/arikkfir/kude/test"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"gopkg.in/yaml.v3"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	err := filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatal(fmt.Errorf("error walking '%s': %w", path, err))
			return nil
		} else if !d.IsDir() {
			return nil
		} else if !strings.HasSuffix(path, ".test") {
			return nil
		} else {
			absPath, err := filepath.Abs(path)
			if err != nil {
				t.Fatal(fmt.Errorf("error creating absolute path from '%s': %w", path, err))
			}

			pwd, err := os.Getwd()
			if err != nil {
				t.Fatal(fmt.Errorf("error getting working directory: %w", err))
			}
			if err := os.Chdir(absPath); err != nil {
				t.Fatal(fmt.Errorf("error changing working directory to '%s': %w", absPath, err))
			}
			defer func() {
				err := os.Chdir(pwd)
				if err != nil {
					t.Error(fmt.Errorf("error changing working directory back to '%s': %w", pwd, err))
				}
			}()

			stdout, _ := test.Capture(true, false, func() {
				commands.RootCmd.SetArgs([]string{"build"})
				err := commands.RootCmd.Execute()
				if err != nil {
					t.Fatal(fmt.Errorf("command failed: %w", err))
				}
			})

			actualFormatted := bytes.Buffer{}
			decoder := yaml.NewDecoder(strings.NewReader(stdout))
			encoder := yaml.NewEncoder(&actualFormatted)
			encoder.SetIndent(2)
			for {
				var data interface{}
				if err := decoder.Decode(&data); err != nil {
					if err == io.EOF {
						break
					}
					t.Fatal(fmt.Errorf("failed decoding YAML: %w", err))
				}
				if err := encoder.Encode(data); err != nil {
					t.Fatal(fmt.Errorf("failed encoding struct: %w", err))
				}
			}

			expectedPath := filepath.Join(absPath, "expected.yaml")
			expectedFile, err := os.Open(expectedPath)
			if err != nil {
				t.Fatal(fmt.Errorf("failed opening expected YAML file at '%s': %w", expectedPath, err))
			}
			expected, err := io.ReadAll(expectedFile)
			if err != nil {
				t.Fatal(fmt.Errorf("failed reading expected YAML file at '%s': %w", expectedPath, err))
			}
			if string(expected) != actualFormatted.String() {
				edits := myers.ComputeEdits(span.URIFromPath("expected"), string(expected), actualFormatted.String())
				diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", string(expected), edits))
				t.Errorf("Incorrect output:\n===\n%s\n===", diff)
			}

			return nil
		}
	})
	if err != nil {
		t.Fatal(fmt.Errorf("failed walking scenarios: %w", err))
	}
}
