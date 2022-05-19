package main

import (
	_ "embed"
	"errors"
	"fmt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
	"text/template"
	"unicode"
)

//go:embed test-template.go.tmpl
var testTemplate string
var testTmpl *template.Template

func main() {
	var err error
	if testTmpl, err = template.New("test").Parse(testTemplate); err != nil {
		log.Fatalf("Failed parsing test template: %v", err)
	}

	walker := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		} else if d.IsDir() {
			return nil
		} else if strings.HasPrefix(d.Name(), "scenario-") && strings.HasSuffix(d.Name(), ".yaml") {
			if err := generateScenarioTest(path); err != nil {
				log.Fatalf("Failed to generate test for '%s': %v", path, err)
			}
		} else if strings.HasPrefix(d.Name(), "scenario") && strings.HasSuffix(d.Name(), "_test.go") {
			dir := filepath.Dir(path)
			scenarioYAMLFileName := strings.TrimSuffix(d.Name(), "_test.go") + ".yaml"
			scenarioYAMLFilePath := filepath.Join(dir, scenarioYAMLFileName)
			if _, err := os.Stat(scenarioYAMLFilePath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					if err := os.Remove(path); err != nil {
						log.Fatalf("Failed removing stale scenario test file at '%s': %v", path, err)
					}
				} else {
					log.Fatalf("Failed inspecting scenario test file at '%s': %v", path, err)
				}
			}
		}
		return nil
	}

	if err := filepath.WalkDir(".", walker); err != nil {
		log.Fatalf("Failed generating sceanrio tests: %v", err)
	}
}

func generateScenarioTest(path string) error {
	dirName, baseName := filepath.Split(path)
	scenario := strings.TrimSuffix(baseName, ".yaml")
	scenario = strings.TrimPrefix(scenario, "scenario-")
	scenario = strings.TrimPrefix(scenario, "test-")

	packageName := filepath.Base(dirName)
	if scenarioRN, err := yaml.ReadFile(path); err != nil {
		return fmt.Errorf("failed to read scenario file '%s': %w", path, err)
	} else if packageNameValue, err := scenarioRN.GetFieldValue("go.package"); err != nil {
		if _, ok := err.(yaml.NoFieldError); !ok {
			return fmt.Errorf("failed to get package name from scenario file '%s': %w", path, err)
		}
	} else if packageNameValue != "" {
		packageName = packageNameValue.(string)
	}

	runes := []rune(scenario)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '_' && !unicode.IsLetter(runes[i]) {
			runes[i] = ' '
		}
	}
	capitalizedWordsScenario := cases.Title(language.AmericanEnglish).String(string(runes))
	camelCaseScenarioName := strings.ReplaceAll(capitalizedWordsScenario, " ", "")

	data := map[string]interface{}{
		"PackageName":        packageName,
		"ScenarioDirname":    dirName,
		"ScenarioBasename":   baseName,
		"ScenarioName":       scenario,
		"ScenarioCamelCased": camelCaseScenarioName,
	}

	targetTestPath := filepath.Join(dirName, fmt.Sprintf("%s_test.go", strings.TrimSuffix(baseName, ".yaml")))
	targetTestFile, err := os.OpenFile(targetTestPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open '%s' for write: %w", targetTestPath, err)
	}
	defer targetTestFile.Close()

	if err := testTmpl.Execute(targetTestFile, data); err != nil {
		return fmt.Errorf("failed generating test for '%s': %w", path, err)
	}

	return nil
}
