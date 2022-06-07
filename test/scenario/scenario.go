package scenario

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	APIVersion = "kude.kfirs.com/v1alpha1"
	Kind       = "Scenario"
)

type Scenario struct {
	Name             string
	Dir              string
	ManifestPath     string
	ExpectedContents string
	ExpectedError    string
}

func OpenScenario(name string, scenarioReader io.Reader) (*Scenario, error) {
	scenario := Scenario{Name: name}

	if dir, err := os.MkdirTemp("", scenario.Name); err != nil {
		return nil, fmt.Errorf("failed to create tempdir: %w", err)
	} else if bytes, err := ioutil.ReadAll(scenarioReader); err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	} else if scenarioRN, err := yaml.Parse(string(bytes)); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	} else if scenarioRN.GetApiVersion() != APIVersion {
		return nil, fmt.Errorf("incorrect scenario API version: expected '%s', got '%s')", APIVersion, scenarioRN.GetApiVersion())
	} else if scenarioRN.GetKind() != "Scenario" {
		return nil, fmt.Errorf("incorrect scenario Kind: expected '%s', got '%s')", Kind, scenarioRN.GetKind())
	} else {
		scenario.Dir = dir
		scenario.ManifestPath = filepath.Join(dir, "kude.yaml")
		if err := yaml.WriteFile(scenarioRN.Field("package").Value, scenario.ManifestPath); err != nil {
			return nil, fmt.Errorf("failed to write package manifest: %w", err)
		}

		resourcesRN := scenarioRN.Field("resources")
		if resourcesRN != nil {
			if fields, err := resourcesRN.Value.Fields(); err != nil {
				return nil, fmt.Errorf("failed to get resources: %w", err)
			} else {
				for _, field := range fields {
					fieldNode := resourcesRN.Value.Field(field)
					if fieldNode == nil {
						return nil, fmt.Errorf("failed to get resource '%s'", field)
					}

					resourceName := fieldNode.Key
					resourceNode := fieldNode.Value
					resourceFilename := resourceName.YNode().Value
					resourceContents := resourceNode.YNode().Value
					targetFile := filepath.Join(dir, resourceFilename)
					targetDir := filepath.Dir(targetFile)
					if err := os.MkdirAll(targetDir, 0755); err != nil {
						return nil, fmt.Errorf("failed creating directory '%s': %w", targetDir, err)
					} else if err := ioutil.WriteFile(targetFile, []byte(resourceContents), 0644); err != nil {
						return nil, fmt.Errorf("failed to write resource '%s': %w", field, err)
					}
				}
			}
		}

		expectedField := scenarioRN.Field("expected")
		if expectedField != nil {
			scenario.ExpectedContents = expectedField.Value.YNode().Value
		}

		expectedErrorField := scenarioRN.Field("expectedError")
		if expectedErrorField != nil {
			scenario.ExpectedError = expectedErrorField.Value.YNode().Value
		}
	}
	return &scenario, nil
}

func (s *Scenario) Close() {
	if err := os.RemoveAll(s.Dir); err != nil {
		log.Printf("failed to remove temp dir '%s': %s", s.Dir, err)
	}
}
