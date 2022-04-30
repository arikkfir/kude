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
	"strings"
	"text/template"
	"unicode"
)

//go:embed test-template.go.tmpl
var testTemplate string

func main() {
	testTmpl, err := loadTestTemplate()
	if err != nil {
		log.Fatalf("Failed to load test template: %v", err)
	}

	err = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		} else if !d.IsDir() {
			return nil
		} else if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".idea" {
				return fs.SkipDir
			} else {
				return generateTest(testTmpl, path)
			}
		} else {
			return nil
		}
	})
	if err != nil {
		log.Fatalf("Failed generating tests: %v", err)
	}
}

func loadTestTemplate() (*template.Template, error) {
	if testTmpl, err := template.New("test").Parse(testTemplate); err != nil {
		return nil, fmt.Errorf("failed parsing test template: %w", err)
	} else {
		return testTmpl, nil
	}
}

func generateTest(testTmpl *template.Template, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to make '%s' an absolute path: %w", path, err)
	}
	kudeYAMLPath := filepath.Join(abs, "kude.yaml")
	if e, err := os.Stat(kudeYAMLPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		} else {
			return fmt.Errorf("failed to stat %s: %w", kudeYAMLPath, err)
		}
	} else if e.IsDir() {
		return fmt.Errorf("expecting '%s' to be a file, not a directory", kudeYAMLPath)
	}

	expectedYAMLPath := filepath.Join(abs, "expected.yaml")
	if e, err := os.Stat(expectedYAMLPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		} else {
			return fmt.Errorf("failed to stat %s: %w", expectedYAMLPath, err)
		}
	} else if e.IsDir() {
		return fmt.Errorf("expecting '%s' to be a file, not a directory", expectedYAMLPath)
	}

	runes := []rune(filepath.Base(path))
	for i := 0; i < len(runes); i++ {
		if runes[i] != '_' && !unicode.IsLetter(runes[i]) {
			runes[i] = ' '
		}
	}
	scenario := cases.Title(language.AmericanEnglish).String(string(runes))
	scenario = strings.ReplaceAll(scenario, " ", "")

	data := map[string]interface{}{
		"ScenarioName": scenario,
	}

	targetTestPath := filepath.Join(abs, "scenario_test.go")
	targetTestFile, err := os.OpenFile(targetTestPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open '%s' for write: %w", targetTestPath, err)
	}
	defer targetTestFile.Close()

	if err := testTmpl.Execute(targetTestFile, data); err != nil {
		return fmt.Errorf("failed generating test for '%s': %w", abs, err)
	}

	return nil
}
