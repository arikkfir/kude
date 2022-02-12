package test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"log"
	"os"
	"time"
)

func BuildFunctionDockerImages() error {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("failed creating Docker client: %w", err)
	}

	functions, err := os.ReadDir("../functions")
	if err != nil {
		return fmt.Errorf("failed listing functions: %w", err)
	}
	for _, functionDir := range functions {
		err := buildFunctionDockerImage(functionDir.Name(), dockerClient)
		if err != nil {
			return fmt.Errorf("failed building function Docker image of '%s': %w", functionDir.Name(), err)
		}
	}
	return nil
}

func buildFunctionDockerImage(function string, dockerClient *client.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()

	log.Println("Creating tar file for Docker image build")
	tar, err := archive.TarWithOptions("../functions/"+function, &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("failed creating tar: %w", err)
	}

	log.Println("Starting Docker image build")
	opts := types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       []string{"ghcr.io/arikkfir/kude/functions/" + function + ":test"},
		Remove:     true,
	}
	res, err := dockerClient.ImageBuild(ctx, tar, opts)
	if err != nil {
		return fmt.Errorf("failed creating Docker image: %w", err)
	}
	defer res.Body.Close()
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		line := scanner.Text()
		var lineData map[string]interface{}
		err := json.Unmarshal([]byte(line), &lineData)
		if err != nil {
			return fmt.Errorf("failed reading Docker image build output: %w", err)
		}
		if lineData["stream"] != nil {
			log.Print(lineData["stream"].(string))
		}
		if lineData["error"] != nil {
			return fmt.Errorf("failed building Docker image: %s (details: %v)", lineData["error"], lineData["errorDetail"])
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed reading Docker image build output: %w", err)
	}

	return nil
}
