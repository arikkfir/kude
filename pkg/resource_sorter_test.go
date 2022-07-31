package kude

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"github.com/arikkfir/kyaml/pkg"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"gopkg.in/yaml.v3"
	"io"
	"sort"
	"strings"
	"testing"
)

//go:embed testdata/resource_sorter_input.yaml
var resourceSorterInputYAML string

//go:embed testdata/resource_sorter_expected.yaml
var resourceSorterExpectedYAML string

func TestResourceSorterByType(t *testing.T) {
	decoder := yaml.NewDecoder(strings.NewReader(resourceSorterInputYAML))
	resources := make([]*kyaml.RNode, 0)
	for {
		n := &yaml.Node{}
		if err := decoder.Decode(n); err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				t.Fatalf("Error decoding YAML: %v", err)
			}
		} else {
			resources = append(resources, &kyaml.RNode{N: n})
		}
	}
	sort.Sort(ByType(resources))

	actualBuffer := bytes.Buffer{}
	encoder := yaml.NewEncoder(&actualBuffer)
	encoder.SetIndent(2)
	for _, r := range resources {
		if err := encoder.Encode(r.N); err != nil {
			t.Fatalf("Error encoding YAML: %v", err)
		}
	}
	encoder.Close()

	expected := strings.TrimSuffix(resourceSorterExpectedYAML, "\n")
	actual := strings.TrimSuffix(actualBuffer.String(), "\n")
	if expected != actual {
		edits := myers.ComputeEdits(span.URIFromPath("expected"), expected, actual)
		diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", expected, edits))
		t.Fatalf("Incorrect output:\n===\n%s\n===", diff)
	}
}
