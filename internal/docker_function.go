package internal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	str "github.com/arikkfir/kude/internal/stream"
	"github.com/arikkfir/kude/pkg"
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

type dockerFunction struct {
	pwd          string
	logger       *logrus.Entry
	dockerClient *client.Client
	name         string
	image        string
	entrypoint   []string
	user         string
	config       map[string]interface{}
}

func (f *dockerFunction) GetName() string {
	return f.name
}

func (f *dockerFunction) pullImage(ctx context.Context) error {
	images, err := f.dockerClient.ImageList(ctx, types.ImageListOptions{
		Filters: filters.NewArgs(
			filters.Arg("reference", f.image),
		),
	})
	if err != nil {
		return fmt.Errorf("failed listing images: %w", err)
	}

	if len(images) > 1 {
		return fmt.Errorf("found multiple matching images")
	} else if len(images) == 1 && !isLatest(&images[0]) {
		return nil
	}

	imagePullReader, err := f.dockerClient.ImagePull(ctx, f.image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed pulling Docker image '%s' of function '%s': %w", f.image, f.name, err)
	}
	defer func(imagePullReader io.ReadCloser) { _ = imagePullReader.Close() }(imagePullReader)

	scanner := bufio.NewScanner(imagePullReader)
	for scanner.Scan() {
		line := scanner.Text()
		var pull map[string]interface{}
		err := json.Unmarshal([]byte(line), &pull)
		if err != nil {
			return fmt.Errorf("failed parsing Docker image pull output: %w", err)
		}
		status := pull["status"]
		delete(pull, "status")
		f.logger.WithField("extra", pull).Trace(status)
	}
	if scanner.Err() != nil {
		return fmt.Errorf("failed parsing Docker image pull output: %w", scanner.Err())
	}
	return nil
}

func (f *dockerFunction) createConfigFile(_ context.Context) (string, func(), error) {
	configFile, err := ioutil.TempFile("", f.name+"-*.yaml")
	if err != nil {
		return "", nil, fmt.Errorf("failed creating temporary file: %w", err)
	}
	cleanup := func() { os.Remove(configFile.Name()) }

	configEncoder := yaml.NewEncoder(configFile)
	err = configEncoder.Encode(f.config)
	if err != nil {
		return "", cleanup, fmt.Errorf("failed marshalling configuration: %w", err)
	}

	err = configEncoder.Close()
	if err != nil {
		return "", cleanup, fmt.Errorf("failed marshalling configuration remainder: %w", err)
	}
	return configFile.Name(), cleanup, nil
}

func (f *dockerFunction) createContainer(ctx context.Context, configFile string) (string, func(), error) {
	containerName := f.name + uuid.New().String()
	cont, err := f.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			AttachStderr: true,
			AttachStdout: true,
			AttachStdin:  true,
			OpenStdin:    true,
			StdinOnce:    true,
			User:         f.user,
			Env: []string{
				"KUDE=true",
				"KUDE_VERSION=" + Version,
			},
			Image:           f.image,
			Entrypoint:      f.entrypoint,
			NetworkDisabled: true,
			Labels: map[string]string{
				"kude":        "true",
				"kudeVersion": Version,
			},
		},
		&container.HostConfig{
			Binds: []string{
				configFile + ":" + pkg.ConfigFile,
			},
		},
		nil,
		nil,
		containerName,
	)
	if err != nil {
		return "", nil, fmt.Errorf("failed creating Docker container: %w", err)
	}
	f.logger = f.logger.WithField("container", cont.ID)
	return cont.ID, func() {
		if err := f.dockerClient.ContainerRemove(ctx, cont.ID, types.ContainerRemoveOptions{}); err != nil {
			f.logger.WithError(err).Error("failed removing container")
		}
	}, nil
}

func (f *dockerFunction) startContainer(ctx context.Context, containerID string) (func(), error) {
	err := f.dockerClient.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed starting Docker container: %w", err)
	}
	return func() {
		if err := f.dockerClient.ContainerStop(ctx, containerID, nil); err != nil {
			f.logger.WithError(err).Error("failed stopping container")
		}
	}, nil
}

func (f *dockerFunction) sendResources(ctx context.Context, containerID string, r io.Reader) error {
	stdinAttachment, err := f.dockerClient.ContainerAttach(ctx, containerID, types.ContainerAttachOptions{
		Stdin:  true,
		Stream: true,
	})
	if err != nil {
		return fmt.Errorf("failed attaching stdin writer to Docker container: %w", err)
	}
	_, err = io.Copy(stdinAttachment.Conn, r)
	if err != nil {
		return fmt.Errorf("failed sending resources to Docker container: %w", err)
	}
	stdinAttachment.Close()
	return nil
}

func (f *dockerFunction) waitForContainer(ctx context.Context, containerID string) (container.ContainerWaitOKBody, error) {
	statusCh, errCh := f.dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	var exit container.ContainerWaitOKBody
	select {
	case err := <-errCh:
		if err != nil {
			return exit, fmt.Errorf("failed waiting for container to exit: %w", err)
		} else {
			return exit, fmt.Errorf("failed waiting for container to exit: nil")
		}
	case exit = <-statusCh:
		// no-op
	}
	return exit, nil
}

func (f *dockerFunction) getContainerOutput(ctx context.Context, containerID string) (io.Reader, error) {
	readers, err := f.dockerClient.ContainerAttach(ctx, containerID, types.ContainerAttachOptions{
		Stdout: true,
		Stderr: true,
		Logs:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed attaching stdout/stderr readers to Docker container: %w", err)
	}
	defer readers.Close()

	//
	// Start copying container output to our stderr and our output pipe
	//
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed creating pipe for returning function results: %w", err)
	}
	_, err = stdcopy.StdCopy(pw, text.NewIndentWriter(f.logger.Writer(), []byte("  -> ")), readers.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed reading stdout/stderr from Docker container: %w", err)
	}
	err = pw.Close()
	if err != nil {
		return nil, fmt.Errorf("failed closing pipe for returning function results: %w", err)
	}
	return pr, nil
}

func (f *dockerFunction) Invoke(ctx context.Context, r io.Reader, w io.Writer) error {
	err := f.pullImage(ctx)
	if err != nil {
		return fmt.Errorf("failed pulling Docker image of function '%s': %w", f.name, err)
	}

	configFile, configFileCleanup, err := f.createConfigFile(ctx)
	defer configFileCleanup()
	if err != nil {
		return fmt.Errorf("failed creating configuration file for function '%s': %w", f.name, err)
	}

	containerID, createContainerCleanup, err := f.createContainer(ctx, configFile)
	defer createContainerCleanup()
	if err != nil {
		return fmt.Errorf("failed creating Docker container for function '%s': %w", f.name, err)
	}

	startContainerCleanup, err := f.startContainer(ctx, containerID)
	defer startContainerCleanup()
	if err != nil {
		return fmt.Errorf("failed starting Docker container for function '%s': %w", f.name, err)
	}

	err = f.sendResources(ctx, containerID, r)
	if err != nil {
		return fmt.Errorf("failed sending resources to Docker container for function '%s': %w", f.name, err)
	}

	exit, err := f.waitForContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed waiting for Docker container to exit for function '%s': %w", f.name, err)
	}

	pr, err := f.getContainerOutput(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed getting output from Docker container for function '%s': %w", f.name, err)
	}

	stream := str.NewStream(f.pwd, handleDirectory, w)
	defer stream.Close()
	err = stream.AddReader(pr)
	if err != nil {
		return fmt.Errorf("failed reading function output: %w", err)
	}

	if exit.Error != nil {
		return fmt.Errorf("container for function '%s' exited with error: %s", f.name, exit.Error.Message)
	} else if exit.StatusCode != 0 {
		return fmt.Errorf("container for function '%s' exited with status code: %d", f.name, exit.StatusCode)
	}
	return nil
}

func newDockerFunction(pwd, name, image string, entrypoint []string, user string, config map[string]interface{}) (*dockerFunction, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed creating Docker client for function '%s': %w", name, err)
	}

	f := &dockerFunction{
		pwd:          pwd,
		logger:       logrus.WithField("function", name),
		dockerClient: dockerClient,
		name:         name,
		image:        image,
		entrypoint:   entrypoint,
		user:         user,
		config:       config,
	}
	return f, nil
}

func isLatest(image *types.ImageSummary) bool {
	for _, tag := range image.RepoTags {
		if tag == "latest" {
			return true
		}
	}
	return false
}
