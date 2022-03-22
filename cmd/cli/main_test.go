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
	root := "../../test"
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Error(fmt.Errorf("error walking '%s': %w", path, err))
			return nil
		} else if !d.IsDir() {
			return nil
		} else if root == path {
			return nil
		} else if !strings.HasSuffix(path, ".test") {
			return nil
		} else {
			absPath, err := filepath.Abs(path)
			if err != nil {
				t.Error(err)
			}

			pwd, err := os.Getwd()
			if err != nil {
				t.Error(err)
			}
			if err := os.Chdir(absPath); err != nil {
				t.Fatal(err)
			}
			defer func() {
				err := os.Chdir(pwd)
				if err != nil {
					t.Error(err)
				}
			}()
			stdout, _ := test.Capture(true, false, func() {
				commands.RootCmd.SetArgs([]string{"build"})
				err := commands.RootCmd.Execute()
				if err != nil {
					t.Error(err)
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
					t.Fatal(err)
				}
				if err := encoder.Encode(data); err != nil {
					t.Fatal(err)
				}
			}

			expectedFile, err := os.Open(filepath.Join(absPath, "expected.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			expected, err := io.ReadAll(expectedFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(expected) != actualFormatted.String() {
				edits := myers.ComputeEdits(span.URIFromPath("expected"), string(expected), actualFormatted.String())
				diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", string(expected), edits))
				t.Errorf("Incorrect output:\n%s", diff)
			}

			return nil
		}
	})
	if err != nil {
		t.Error(err)
	}
}
