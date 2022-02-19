package internal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"
	"github.com/kr/text"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"os"
)

type Function struct {
	pwd        string
	Name       string                 `json:"name"`
	Image      string                 `json:"image"`
	Entrypoint []string               `json:"entrypoint,omitempty"`
	User       string                 `json:"user,omitempty"`
	Config     map[string]interface{} `json:",inline"`
}

func (f *Function) invokeFunction(r io.Reader, w io.Writer) error {
	ctx := context.Background()
	logger := logrus.WithField("function", f.Name)

	//
	// Connect to Docker engine
	//
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("failed creating Docker client for function '%s': %w", f.Name, err)
	}

	//
	// Prepare function config temporary file
	//
	configFile, err := ioutil.TempFile("", f.Name+"-*.yaml")
	if err != nil {
		return fmt.Errorf("failed creating temporary config file for function '%s': %w", f.Name, err)
	}
	defer func(name string) { _ = os.Remove(name) }(configFile.Name())
	configEncoder := yaml.NewEncoder(configFile)
	err = configEncoder.Encode(f.Config)
	if err != nil {
		return fmt.Errorf("failed marshalling configuration of function '%s': %w", f.Name, err)
	}
	err = configEncoder.Close()
	if err != nil {
		return fmt.Errorf("failed marshalling configuration of function '%s': %w", f.Name, err)
	}

	//
	// Pull function image
	//
	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{
		All: false,
		Filters: filters.NewArgs(
			filters.Arg("reference", f.Image),
		),
	})
	if err != nil {
		return fmt.Errorf("failed listing images for function '%s': %w", f.Name, err)
	}
	pull := true
	if len(images) > 1 {
		return fmt.Errorf("found multiple images for function '%s'", f.Name)
	} else if len(images) == 1 {
		pull = false
		for _, tag := range images[0].RepoTags {
			if tag == "latest" {
				pull = true
			}
		}
	}
	if pull {
		imagePullReader, err := dockerClient.ImagePull(ctx, f.Image, types.ImagePullOptions{})
		if err != nil {
			return fmt.Errorf("failed pulling Docker image '%s' of function '%s': %w", f.Image, f.Name, err)
		}
		defer func(imagePullReader io.ReadCloser) { _ = imagePullReader.Close() }(imagePullReader)
		scanner := bufio.NewScanner(imagePullReader)
		for scanner.Scan() {
			line := scanner.Text()
			var pull map[string]interface{}
			err := json.Unmarshal([]byte(line), &pull)
			if err != nil {
				logger.WithError(err).WithField("line", line).Error("failed parsing Docker image pull output")
				break
			}
			status := pull["status"]
			delete(pull, "status")
			logger.WithField("extra", pull).Debug(status)
		}
		if scanner.Err() != nil {
			logger.WithError(err).Error("failed parsing Docker image pull output")
		}
		logger.WithField("image", f.Image).Debug("Image pulled")
	} else {
		logger.WithField("image", f.Image).Debug("Image already present (and not tagged 'latest')")
	}

	//
	// Create container
	//
	cont, err := dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			AttachStderr: true,
			AttachStdout: true,
			AttachStdin:  true,
			OpenStdin:    true,
			StdinOnce:    true,
			User:         f.User,
			Env: []string{
				"KUDE=true",
				"KUDE_VERSION=" + Version,
			},
			Image:           f.Image,
			Entrypoint:      f.Entrypoint,
			NetworkDisabled: true,
			Labels: map[string]string{
				"kude":        "true",
				"kudeVersion": Version,
			},
		},
		&container.HostConfig{
			Binds: []string{
				configFile.Name() + ":/etc/kude/function/config.yaml",
			},
		},
		nil,
		nil,
		f.Name+uuid.New().String(),
	)
	if err != nil {
		return fmt.Errorf("failed creating Docker container for function '%s': %w", f.Name, err)
	}
	defer func() {
		if err := dockerClient.ContainerRemove(ctx, cont.ID, types.ContainerRemoveOptions{}); err != nil {
			logger.WithError(err).Error("failed removing container")
		}
	}()
	logger = logger.WithField("container", cont.ID)
	logger.Debug("Container created")

	//
	// Start container
	//
	err = dockerClient.ContainerStart(ctx, cont.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("failed starting Docker container of function '%s': %w", f.Name, err)
	}
	defer func() {
		if err := dockerClient.ContainerStop(ctx, cont.ID, nil); err != nil {
			logger.WithError(err).Error("failed stopping container")
		}
	}()
	logger.Debug("Started container")

	//
	// Attach to container & send config & resources to container stdin
	//
	stdinAttachment, err := dockerClient.ContainerAttach(ctx, cont.ID, types.ContainerAttachOptions{
		Stdin:  true,
		Stream: true,
	})
	if err != nil {
		return fmt.Errorf("failed attaching stdin writer to Docker container of function '%s': %w", f.Name, err)
	}
	_, err = io.Copy(stdinAttachment.Conn, r)
	if err != nil {
		return fmt.Errorf("failed sending resources to Docker container of function '%s': %w", f.Name, err)
	}
	stdinAttachment.Close()

	//
	// Wait for container to finish
	//
	logger.Debug("Waiting for Docker container to finish...")
	statusCh, errCh := dockerClient.ContainerWait(ctx, cont.ID, container.WaitConditionNotRunning)
	var exit container.ContainerWaitOKBody
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("failed waiting for container of function '%s' to exit: %w", f.Name, err)
		} else {
			return fmt.Errorf("failed waiting for container of function '%s' to exit: nil", f.Name)
		}
	case exit = <-statusCh:
		logger.WithField("exitCode", exit.StatusCode).Debug("Container exited")
	}

	//
	// Prepare container output reader
	//
	logger.Debug("Attaching stdout/stderr readers to Docker container...")
	readers, err := dockerClient.ContainerAttach(ctx, cont.ID, types.ContainerAttachOptions{
		Stdout: true,
		Stderr: true,
		Logs:   true,
	})
	if err != nil {
		return fmt.Errorf("failed attaching stdout/stderr readers to Docker container of function '%s': %w", f.Name, err)
	}
	defer readers.Close()

	//
	// Start copying container output to our stderr and our output pipe
	//
	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed creating pipe for returning function results: %w", err)
	}
	_, err = stdcopy.StdCopy(pw, text.NewIndentWriter(logger.Writer(), []byte("  -> ")), readers.Reader)
	if err != nil {
		return fmt.Errorf("failed reading stdout/stderr from Docker container of function '%s': %w", f.Name, err)
	}
	err = pw.Close()
	if err != nil {
		return fmt.Errorf("failed closing pipe for returning function results: %w", err)
	}

	//
	// Read all YAML output from the function, and re-encode it to preserve consistent formatting
	//
	stream := NewStream(f.pwd, w)
	defer stream.Close()
	err = stream.addReader(pr)
	if err != nil {
		return fmt.Errorf("failed reading function output: %w", err)
	}

	//
	// Fail if container failed
	//
	if exit.Error != nil {
		return fmt.Errorf("container for function '%s' exited with error: %s", f.Name, exit.Error.Message)
	} else if exit.StatusCode != 0 {
		return fmt.Errorf("container for function '%s' exited with status code: %d", f.Name, exit.StatusCode)
	}
	return nil
}
