package test

import (
	"fmt"
	"os"
	"os/exec"
)

func BuildFunctionDockerImages() error {
	functions, err := os.ReadDir("../cmd/functions")
	if err != nil {
		return fmt.Errorf("failed listing functions: %w", err)
	}
	for _, functionDir := range functions {
		if functionDir.IsDir() {
			err := buildFunctionDockerImage(functionDir.Name())
			if err != nil {
				return fmt.Errorf("failed building function Docker image of '%s': %w", functionDir.Name(), err)
			}
		}
	}
	return nil
}

func buildFunctionDockerImage(function string) error {
	imageName := fmt.Sprintf("ghcr.io/arikkfir/kude/functions/%s:test", function)
	dockerFilePath := fmt.Sprintf("../cmd/functions/%s/Dockerfile", function)
	cmd := exec.Command("docker", "build", "-t", imageName, "-f", dockerFilePath, "--build-arg", "function="+function, "../")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed building Docker image: %w", err)
	}
	return nil
}
