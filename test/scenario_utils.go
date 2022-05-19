package test

import (
	"bytes"
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
	"testing"
)

const (
	ScenarioAPIVersion = "kude.kfirs.com/v1alpha1"
	ScenarioKind       = "Scenario"
)

type PackageFactory func(logger *log.Logger, pwd string, manifestReader io.Reader, output io.Writer) (pkg.Package, error)

func extractScenario(scenarioName, scenarioYAML string) (dir, manifestPath, expectedPath, expectedError string, err error) {
	if dir, err = os.MkdirTemp("", scenarioName); err != nil {
		return "", "", "", "", fmt.Errorf("failed to create tempdir: %w", err)
	}

	scenarioRN := yaml.MustParse(scenarioYAML)
	if scenarioRN.GetApiVersion() != ScenarioAPIVersion {
		return "", "", "", "", fmt.Errorf("incorrect scenario API version: expected '%s', got '%s')", ScenarioAPIVersion, scenarioRN.GetApiVersion())
	} else if scenarioRN.GetKind() != "Scenario" {
		return "", "", "", "", fmt.Errorf("incorrect scenario Kind: expected '%s', got '%s')", ScenarioKind, scenarioRN.GetKind())
	}

	manifestPath = filepath.Join(dir, "kude.yaml")
	pkgRN := scenarioRN.Field("package")
	if err := yaml.WriteFile(pkgRN.Value, manifestPath); err != nil {
		return "", "", "", "", fmt.Errorf("failed to write package manifest: %w", err)
	}

	resourcesRN := scenarioRN.Field("resources")
	if resourcesRN != nil {
		if fields, err := resourcesRN.Value.Fields(); err != nil {
			return "", "", "", "", fmt.Errorf("failed to get resources: %w", err)
		} else {
			for _, field := range fields {
				fieldNode := resourcesRN.Value.Field(field)
				if fieldNode == nil {
					return "", "", "", "", fmt.Errorf("failed to get resource '%s'", field)
				}

				resourceName := fieldNode.Key
				resourceNode := fieldNode.Value
				resourceFilename := resourceName.YNode().Value
				resourceContents := resourceNode.YNode().Value
				targetFile := filepath.Join(dir, resourceFilename)
				targetDir := filepath.Dir(targetFile)
				if err := os.MkdirAll(targetDir, 0755); err != nil {
					return "", "", "", "", fmt.Errorf("failed creating directory '%s': %w", targetDir, err)
				} else if err := ioutil.WriteFile(targetFile, []byte(resourceContents), 0644); err != nil {
					return "", "", "", "", fmt.Errorf("failed to write resource '%s': %w", field, err)
				}
			}
		}
	}

	expectedField := scenarioRN.Field("expected")
	if expectedField != nil {
		expectedPath = filepath.Join(dir, "expected.yaml")
		expectedFieldValue := expectedField.Value
		expectedFieldValueYNode := expectedFieldValue.YNode()
		expectedYAML := expectedFieldValueYNode.Value
		if err := ioutil.WriteFile(expectedPath, []byte(expectedYAML), 0644); err != nil {
			return "", "", "", "", fmt.Errorf("failed to write expected contents file: %w", err)
		}
	}

	expectedErrorField := scenarioRN.Field("expectedError")
	if expectedErrorField != nil {
		expectedErrorFieldValue := expectedErrorField.Value
		expectedErrorFieldValueYNode := expectedErrorFieldValue.YNode()
		expectedError = expectedErrorFieldValueYNode.Value
	}

	return
}

func formatYAML(yamlString io.Reader) (string, error) {
	const formatYAMLFailureMessage = `%s: %w
======
%s
======`

	formatted := bytes.Buffer{}
	decoder := yaml.NewDecoder(yamlString)
	encoder := yaml.NewEncoder(&formatted)
	encoder.SetIndent(2)
	for {
		var data interface{}
		if err := decoder.Decode(&data); err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf(formatYAMLFailureMessage, "failed decoding YAML", err, yamlString)
		} else if err := encoder.Encode(data); err != nil {
			return "", fmt.Errorf(formatYAMLFailureMessage, "failed encoding struct", err, yamlString)
		}
	}
	return formatted.String(), nil
}

func RunScenario(_ *testing.T, scenarioName, scenarioYAML string, packageFactory PackageFactory) (err error) {
	pwd, manifestPath, expectedPath, expectedError, err := extractScenario(scenarioName, scenarioYAML)
	if err != nil {
		return fmt.Errorf("failed to extract scenario: %w", err)
	}
	defer func() {
		if err == nil && pwd != "" && (strings.HasPrefix(pwd, "/var/") || strings.HasPrefix(pwd, "/tmp/")) {
			os.RemoveAll(pwd)
		}
	}()

	manifestReader, err := os.Open(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to open package manifest at '%s': %w", manifestPath, err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	defer func() {
		const msg = `%w
==- STDERR -===============
%s
==- STDOUT -===============
%s
///////////////////////////`
		if err != nil {
			err = fmt.Errorf(msg, err, stderr, stdout)
		}
	}()

	logger := log.New(stderr, log.Prefix(), log.Flags())
	p, err := packageFactory(logger, pwd, manifestReader, stdout)
	if err != nil {
		if expectedError != "" {
			if match, matchErr := regexp.Match(expectedError, []byte(err.Error())); matchErr != nil {
				return fmt.Errorf("failed to compare expected error: %w", matchErr)
			} else if match {
				// as expected
				return nil
			} else {
				return fmt.Errorf("incorrect error received during package creation! expected '%s', received: %w", expectedError, err)
			}
		} else {
			return fmt.Errorf("failed to build package: %w", err)
		}
	}

	if err := p.Execute(); err != nil {
		if expectedError != "" {
			if match, matchErr := regexp.Match(expectedError, []byte(err.Error())); matchErr != nil {
				return fmt.Errorf("failed to compare expected error: %w", matchErr)
			} else if match {
				// as expected
				return nil
			} else {
				return fmt.Errorf("incorrect error received during package execution! expected '%s', received: %w", expectedError, err)
			}
		} else {
			return fmt.Errorf("failed to execute package: %w", err)
		}
	}

	actual, err := formatYAML(stdout)
	if err != nil {
		return fmt.Errorf("failed to format YAML output: %w", err)
	}

	if expectedPath != "" {
		if expectedFile, err := os.Open(expectedPath); err != nil {
			return fmt.Errorf("failed opening expected YAML file at '%s': %w", expectedPath, err)

		} else if rawExpected, err := io.ReadAll(expectedFile); err != nil {
			return fmt.Errorf("failed reading expected YAML file at '%s': %w", expectedPath, err)

		} else if expected, err := formatYAML(strings.NewReader(string(rawExpected))); err != nil {
			return fmt.Errorf("failed to format expected YAML file at '%s': %w", expectedPath, err)

		} else if strings.TrimSuffix(expected, "\n") != strings.TrimSuffix(actual, "\n") {
			edits := myers.ComputeEdits(span.URIFromPath("expected"), string(expected), actual)
			diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", string(expected), edits))
			return fmt.Errorf("Incorrect output:\n===\n%s\n===", diff)
		}
	}

	return nil
}
