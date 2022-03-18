package internal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/blang/semver"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
)

var mountKeyRegex = regexp.MustCompile(`^kude\.kfirs\.com/mount:(.+)`)

type dockerFunction struct {
	pwd          string
	logger       *logrus.Entry
	bindsRegexp  *regexp.Regexp
	binds        []string
	name         string
	image        string
	_image       types.ImageSummary
	entrypoint   []string
	user         string
	allowNetwork bool
	config       map[string]interface{}
}

func (f *dockerFunction) pullImage(ctx context.Context, dockerClient *client.Client) error {
	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{
		Filters: filters.NewArgs(
			filters.Arg("reference", f.image),
		),
	})
	if err != nil {
		return fmt.Errorf("failed listing images: %w", err)
	}

	if len(images) > 1 {
		return fmt.Errorf("found multiple matching images")
	} else if len(images) == 0 || isLatest(&images[0]) {
		imagePullReader, err := dockerClient.ImagePull(ctx, f.image, types.ImagePullOptions{})
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

		images, err = dockerClient.ImageList(ctx, types.ImageListOptions{
			Filters: filters.NewArgs(
				filters.Arg("reference", f.image),
			),
		})
		if err != nil {
			return fmt.Errorf("failed listing images: %w", err)
		} else if len(images) != 1 {
			return fmt.Errorf("expected images list length to be 1")
		}
	}

	minVersionValue, ok := images[0].Labels["kude.kfirs.com/minimum-version"]
	if ok {
		minVersion, err := semver.Parse(minVersionValue)
		if err != nil {
			return fmt.Errorf("failed parsing minimum version '%s': %w", minVersionValue, err)
		}
		if pkg.GetVersion().LT(minVersion) {
			//goland:noinspection GoErrorStringFormat
			return fmt.Errorf("Kude version '%s' or higher is required", minVersionValue)
		}
	}

	f._image = images[0]
	return nil
}

func (f *dockerFunction) createConfigFile(_ context.Context) (string, func(), error) {
	configFileName := f.name
	configFileName = strings.ReplaceAll(configFileName, ":", "_")
	configFileName = strings.ReplaceAll(configFileName, "/", "_")
	configFileName = strings.ReplaceAll(configFileName, "\\", "_")
	configFileName = strings.ReplaceAll(configFileName, "?", "_")
	configFileName = strings.ReplaceAll(configFileName, "*", "_")
	configFileName = strings.ReplaceAll(configFileName, "(", "_")
	configFileName = strings.ReplaceAll(configFileName, ")", "_")
	configFile, err := ioutil.TempFile("", configFileName+"-*.yaml")
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

func (f *dockerFunction) collectBindsFromImageLabels() error {
	configYAMLBytes, err := yaml.Marshal(f.config)
	if err != nil {
		return fmt.Errorf("failed marshalling configuration: %w", err)
	}

	var configYAMLNode yaml.Node
	err = yaml.Unmarshal(configYAMLBytes, &configYAMLNode)
	if err != nil {
		return fmt.Errorf("failed unmarshalling configuration: %w", err)
	}

	for k, v := range f._image.Labels {
		subMatches := mountKeyRegex.FindStringSubmatch(k)
		if subMatches != nil && len(subMatches) == 2 {
			yamlPath, err := yamlpath.NewPath(v)
			if err != nil {
				return fmt.Errorf("failed parsing YAML path '%s': %w", v, err)
			}
			nodes, err := yamlPath.Find(&configYAMLNode)
			if err != nil {
				return fmt.Errorf("failed finding YAML path '%s': %w", v, err)
			}
			for _, node := range nodes {
				if node.Kind != yaml.ScalarNode {
					return fmt.Errorf("YAML path '%s' is not a scalar", v)
				}
				localPath := node.Value
				if strings.HasPrefix(localPath, "/") || strings.Contains(localPath, "..") {
					return fmt.Errorf("non-local paths are disallowed for mounting ('%s'): %w", localPath, err)
				}
				resolvedPath := filepath.Join(f.pwd, localPath)
				_, err := os.Stat(resolvedPath)
				if err != nil {
					return fmt.Errorf("failed statting local path '%s': %w", localPath, err)
				}
				f.binds = append(f.binds, fmt.Sprintf("%s:%s", resolvedPath, "/workspace/"+localPath))
			}
		}
	}
	return nil
}

func (f *dockerFunction) createContainer(ctx context.Context, dockerClient *client.Client, configFile string) (string, func(), error) {
	err := f.collectBindsFromImageLabels()
	if err != nil {
		return "", nil, fmt.Errorf("failed collecting mount files: %w", err)
	}
	f.binds = append(f.binds, configFile+":"+pkg.ConfigFile)

	tempDir := filepath.Join(f.pwd, ".kude", "temp")
	err = os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		return "", nil, fmt.Errorf("failed creating temp directory '%s': %w", tempDir, err)
	}
	f.binds = append(f.binds, tempDir+":/workspace/temp")

	containerName := f.name + uuid.New().String()
	cont, err := dockerClient.ContainerCreate(
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
				"KUDE_VERSION=" + pkg.GetVersion().String(),
			},
			Image:           f.image,
			Entrypoint:      f.entrypoint,
			NetworkDisabled: !f.allowNetwork,
			Labels: map[string]string{
				"kude":        "true",
				"kudeVersion": pkg.GetVersion().String(),
			},
		},
		&container.HostConfig{Binds: f.binds},
		nil,
		nil,
		containerName,
	)
	if err != nil {
		return "", nil, fmt.Errorf("failed creating Docker container: %w", err)
	}
	f.logger = f.logger.WithField("container", cont.ID)
	return cont.ID, func() {
		if err := dockerClient.ContainerRemove(ctx, cont.ID, types.ContainerRemoveOptions{}); err != nil {
			f.logger.WithError(err).Error("failed removing container")
		}
	}, nil
}

func (f *dockerFunction) startContainer(ctx context.Context, dockerClient *client.Client, containerID string) (func(), error) {
	err := dockerClient.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed starting Docker container: %w", err)
	}
	return func() {
		if err := dockerClient.ContainerStop(ctx, containerID, nil); err != nil {
			f.logger.WithError(err).Error("failed stopping container")
		}
	}, nil
}

func (f *dockerFunction) sendResources(ctx context.Context, dockerClient *client.Client, containerID string, rns []*kyaml.RNode) error {
	stdinAttachment, err := dockerClient.ContainerAttach(ctx, containerID, types.ContainerAttachOptions{
		Stdin:  true,
		Stream: true,
	})
	if err != nil {
		return fmt.Errorf("failed attaching stdin writer to Docker container: %w", err)
	}
	err = kio.ByteWriter{Writer: stdinAttachment.Conn}.Write(rns)
	if err != nil {
		return fmt.Errorf("failed sending resources to Docker container: %w", err)
	}
	stdinAttachment.Close()
	return nil
}

func (f *dockerFunction) waitForContainer(ctx context.Context, dockerClient *client.Client, containerID string) (container.ContainerWaitOKBody, error) {
	statusCh, errCh := dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
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

func (f *dockerFunction) getContainerOutput(ctx context.Context, dockerClient *client.Client, containerID string) (io.Reader, error) {
	readers, err := dockerClient.ContainerAttach(ctx, containerID, types.ContainerAttachOptions{
		Stdout: true,
		Stderr: true,
		Logs:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed attaching stdout/stderr readers to Docker container: %w", err)
	}
	defer readers.Close()

	// Start copying container output to our stderr and our output pipe
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed creating pipe for returning function results: %w", err)
	}
	_, err = stdcopy.StdCopy(pw, os.Stderr, readers.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed reading stdout/stderr from Docker container: %w", err)
	}
	err = pw.Close()
	if err != nil {
		return nil, fmt.Errorf("failed closing pipe for returning function results: %w", err)
	}
	return pr, nil
}

func (f *dockerFunction) Filter(rns []*kyaml.RNode) ([]*kyaml.RNode, error) {
	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed creating Docker client for function '%s': %w", f.name, err)
	}

	err = f.pullImage(ctx, dockerClient)
	if err != nil {
		return nil, fmt.Errorf("failed pulling Docker image of function '%s': %w", f.name, err)
	}

	configFile, configFileCleanup, err := f.createConfigFile(ctx)
	if configFileCleanup != nil {
		defer configFileCleanup()
	}
	if err != nil {
		return nil, fmt.Errorf("failed creating configuration file for function '%s': %w", f.name, err)
	}

	containerID, createContainerCleanup, err := f.createContainer(ctx, dockerClient, configFile)
	if createContainerCleanup != nil {
		defer createContainerCleanup()
	}
	if err != nil {
		return nil, fmt.Errorf("failed creating Docker container for function '%s': %w", f.name, err)
	}

	startContainerCleanup, err := f.startContainer(ctx, dockerClient, containerID)
	if startContainerCleanup != nil {
		defer startContainerCleanup()
	}
	if err != nil {
		return nil, fmt.Errorf("failed starting Docker container for function '%s': %w", f.name, err)
	}

	err = f.sendResources(ctx, dockerClient, containerID, rns)
	if err != nil {
		return nil, fmt.Errorf("failed sending resources to Docker container for function '%s': %w", f.name, err)
	}

	exit, err := f.waitForContainer(ctx, dockerClient, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for Docker container to exit for function '%s': %w", f.name, err)
	}

	pr, err := f.getContainerOutput(ctx, dockerClient, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed getting output from Docker container for function '%s': %w", f.name, err)
	}

	rns, err = (&kio.ByteReader{Reader: pr}).Read()
	if err != nil {
		return nil, fmt.Errorf("failed reading function output: %w", err)
	}

	if exit.Error != nil {
		return nil, fmt.Errorf("container for function '%s' exited with error: %s", f.name, exit.Error.Message)
	} else if exit.StatusCode != 0 {
		return nil, fmt.Errorf("container for function '%s' exited with status code: %d", f.name, exit.StatusCode)
	}
	return rns, nil
}
