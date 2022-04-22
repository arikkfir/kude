package internal

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
	"time"
)

var containerStopTimeout = 30 * time.Second

func isLatest(image *types.ImageSummary) bool {
	for _, tag := range image.RepoTags {
		if tag == "latest" {
			return true
		}
	}
	return false
}

type dockerFunction struct {
	logger       *log.Logger
	pwd          string
	name         string
	image        string
	entrypoint   []string
	user         string
	workDir      string
	allowNetwork bool
	config       map[string]interface{}
	mounts       []string
	timeout      time.Duration
}

func (f *dockerFunction) Filter(rns []*kyaml.RNode) ([]*kyaml.RNode, error) {
	f.logger.Printf("Invoking function '%s' (%s)", f.image, f.name)
	logger := log.New(f.logger.Writer(), f.logger.Prefix()+"--> ", f.logger.Flags())
	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed creating Docker client for function '%s': %w", f.name, err)
	}

	////////////////////////////////////////////////////////////////////////////
	// PULL IMAGE
	////////////////////////////////////////////////////////////////////////////
	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", f.image)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed pulling image: %w", err)
	} else if len(images) > 1 {
		return nil, fmt.Errorf("found multiple matching images")
	} else if len(images) == 0 || isLatest(&images[0]) {
		logger.Printf("Pulling image '%s'", f.image)
		r, err := dockerClient.ImagePull(ctx, f.image, types.ImagePullOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed pulling image: %w", err)
		}
		defer r.Close()
		pullLog := log.New(logger.Writer(), "---"+logger.Prefix(), logger.Flags())
		s := bufio.NewScanner(r)
		for s.Scan() {
			line := s.Text()
			var pull map[string]interface{}
			if err := json.Unmarshal([]byte(line), &pull); err != nil {
				return nil, fmt.Errorf("failed parsing image pull output: %w", err)
			}
			pullLog.Println(pull["status"])
		}
		if s.Err() != nil {
			return nil, fmt.Errorf("failed parsing image pull output: %w", s.Err())
		}
	}

	////////////////////////////////////////////////////////////////////////////
	// CREATE CONFIG FILE
	////////////////////////////////////////////////////////////////////////////
	configFile, err := ioutil.TempFile("", "*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed creating temporary file: %w", err)
	}
	defer os.Remove(configFile.Name())
	configEncoder := yaml.NewEncoder(configFile)
	if err := configEncoder.Encode(f.config); err != nil {
		return nil, fmt.Errorf("failed marshalling configuration: %w", err)
	} else if err := configEncoder.Close(); err != nil {
		return nil, fmt.Errorf("failed marshalling configuration remainder: %w", err)
	}
	f.mounts = append(f.mounts, configFile.Name()+":"+pkg.ConfigFile)

	////////////////////////////////////////////////////////////////////////////
	// PREPARE WORKSPACE TEMP DIRECTORY
	////////////////////////////////////////////////////////////////////////////
	tempDir := filepath.Join(f.pwd, ".kude", "temp")
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed creating temp directory '%s': %w", tempDir, err)
	}
	f.mounts = append(f.mounts, tempDir+":/workspace/temp")

	////////////////////////////////////////////////////////////////////////////
	// PREPARE BINDS
	////////////////////////////////////////////////////////////////////////////
	var binds []string
	for _, mount := range f.mounts {
		local, remote, found := strings.Cut(mount, ":")
		if local == "" {
			return nil, fmt.Errorf("invalid mount format: %s", mount)
		} else if !found {
			remote = local
		}
		if !filepath.IsAbs(local) {
			local = filepath.Join(f.pwd, local)
		}
		if _, err := os.Stat(local); errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("could not find '%s'", local)
		} else if err != nil {
			return nil, fmt.Errorf("failed stat for '%s': %w", local, err)
		}
		if !filepath.IsAbs(remote) {
			remote = filepath.Join("/workspace", remote)
		}
		logger.Printf("Mounting '%s' as '%s'", local, remote)
		binds = append(binds, local+":"+remote)
	}

	////////////////////////////////////////////////////////////////////////////
	// CREATE CONTAINER
	////////////////////////////////////////////////////////////////////////////
	logger.Printf("Creating container")
	cont, err := dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			AttachStderr:    true,
			AttachStdout:    true,
			AttachStdin:     true,
			OpenStdin:       true,
			StdinOnce:       true,
			Tty:             false, // Important to disable this, so that the output logs are multiplexed (stdout/stderr)
			User:            f.user,
			WorkingDir:      f.workDir,
			Env:             []string{"KUDE=true", "KUDE_VERSION=" + pkg.GetVersion().String()},
			Image:           f.image,
			Entrypoint:      f.entrypoint,
			NetworkDisabled: !f.allowNetwork,
			Labels:          map[string]string{"kude": "true", "kudeVersion": pkg.GetVersion().String()},
		},
		&container.HostConfig{Binds: binds},
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("failed creating container: %w", err)
	}
	defer func() {
		if err := dockerClient.ContainerRemove(ctx, cont.ID, types.ContainerRemoveOptions{}); err != nil {
			logger.Printf("Failed removing container: %v", err)
		}
	}()

	////////////////////////////////////////////////////////////////////////////
	// START CONTAINER
	////////////////////////////////////////////////////////////////////////////
	logger.Printf("Starting container")
	if err := dockerClient.ContainerStart(ctx, cont.ID, types.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("failed starting container: %w", err)
	}
	defer func() {
		if err := dockerClient.ContainerStop(ctx, cont.ID, &containerStopTimeout); err != nil {
			logger.Printf("Failed stopping container: %v", err)
		}
	}()

	////////////////////////////////////////////////////////////////////////////
	// PUSH GIVEN RESOURCES INTO CONTAINER stdin
	////////////////////////////////////////////////////////////////////////////
	logger.Println("Pushing resources to container...")
	go func() {
		hijack, err := dockerClient.ContainerAttach(ctx, cont.ID, types.ContainerAttachOptions{Stdin: true, Stream: true})
		if err != nil {
			logger.Fatalf("Failed attaching to container stdin: %v", err)
		}
		defer hijack.Close()

		err = kio.ByteWriter{Writer: hijack.Conn}.Write(rns)
		if err != nil {
			logger.Printf("Failed sending resources to Docker container: %v", err)
		}
	}()

	////////////////////////////////////////////////////////////////////////////
	// START READING RESOURCES FROM CONTAINER stdout, AND PIPING ITS stderr
	////////////////////////////////////////////////////////////////////////////
	logger.Println("Reading resources from container...")
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed creating pipe for returning function results: %w", err)
	}
	logs, err := dockerClient.ContainerLogs(ctx, cont.ID, types.ContainerLogsOptions{
		Follow:     true,
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "all",
	})
	if err != nil {
		return nil, fmt.Errorf("failed attaching to container to read stdout/stderr: %w", err)
	}
	defer logs.Close()
	go func() {
		if _, err := stdcopy.StdCopy(pw, &logWriter{logger}, logs); err != nil {
			logger.Printf("Failed copying container output: %v", err)
		}
		if err := pw.Close(); err != nil {
			logger.Fatalf("Failed closing container output writer: %v", err)
		}
	}()

	////////////////////////////////////////////////////////////////////////////
	// READ RESOURCES INTO YAML RESOURCE NODES
	////////////////////////////////////////////////////////////////////////////
	logger.Println("Parsing & validating resources...")
	outputRNs, err := (&kio.ByteReader{Reader: pr}).Read()
	if err != nil {
		return nil, fmt.Errorf("failed reading resources: %w", err)
	}

	////////////////////////////////////////////////////////////////////////////
	// WAIT FOR CONTAINER TO EXIT
	////////////////////////////////////////////////////////////////////////////
	logger.Println("Waiting for container to exit...")
	// TODO: wait on channels for a few seconds, and only then print "Waiting for container to exit..." (reduce unnecessary logs)
	statusCh, errCh := dockerClient.ContainerWait(ctx, cont.ID, container.WaitConditionNotRunning)
	var exit container.ContainerWaitOKBody
	select {
	case err := <-errCh:
		return nil, fmt.Errorf("failed waiting for container to exit: %w", err)
	case exit = <-statusCh:
		err := exit.Error
		if err != nil {
			return nil, fmt.Errorf("failed waiting for container to exit: %s", err.Message)
		} else if exit.StatusCode != 0 {
			return nil, fmt.Errorf("container failed with exit code %d", exit.StatusCode)
		}
	}
	return outputRNs, nil
}
