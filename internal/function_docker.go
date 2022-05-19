package internal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"io"
	"log"
	"sync/atomic"
	"time"
)

var containerStopTimeout = 30 * time.Second

type DockerFunction struct {
	Image      string
	Entrypoint []string
	User       string
	Workdir    string
	Network    bool
	Mounts     []string
	logger     *log.Logger
	pwd        string
	cacheDir   string
	tempDir    string
}

func (f *DockerFunction) Configure(logger *log.Logger, pwd, cacheDir, tempDir string) error {
	f.logger = logger
	f.pwd = pwd
	f.cacheDir = cacheDir
	f.tempDir = tempDir
	return nil
}

func (f *DockerFunction) isImageWithLatestTag(image *types.ImageSummary) bool {
	for _, tag := range image.RepoTags {
		if tag == "latest" {
			return true
		}
	}
	return false
}

func (f *DockerFunction) Invoke(r io.Reader, w io.Writer) error {
	ctx := context.Background()

	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("failed creating Docker client: %w", err)
	}

	////////////////////////////////////////////////////////////////////////////
	// PULL IMAGE
	////////////////////////////////////////////////////////////////////////////
	f.logger.Printf("Searching for image '%s'...", f.Image)
	imageListFilters := filters.NewArgs(filters.Arg("reference", f.Image))
	if images, err := dockerClient.ImageList(ctx, types.ImageListOptions{Filters: imageListFilters}); err != nil {
		return fmt.Errorf("failed pulling image: %w", err)
	} else if len(images) > 1 {
		return fmt.Errorf("found multiple matching images")
	} else if len(images) == 0 || f.isImageWithLatestTag(&images[0]) {
		f.logger.Printf("Pulling image '%s'...", f.Image)
		r, err := dockerClient.ImagePull(ctx, f.Image, types.ImagePullOptions{})
		if err != nil {
			return fmt.Errorf("failed pulling image: %w", err)
		}
		defer r.Close()
		pullLog := ChildLogger(f.logger)
		s := bufio.NewScanner(r)
		for s.Scan() {
			line := s.Text()
			var pull map[string]interface{}
			if err := json.Unmarshal([]byte(line), &pull); err != nil {
				return fmt.Errorf("failed parsing image pull output: %w", err)
			}
			pullLog.Println(pull["status"])
		}
		if s.Err() != nil {
			return fmt.Errorf("failed parsing image pull output: %w", s.Err())
		}
	}

	////////////////////////////////////////////////////////////////////////////
	// CREATE CONTAINER
	////////////////////////////////////////////////////////////////////////////
	f.logger.Printf("Creating container")
	workdir := f.Workdir
	if workdir == "" {
		workdir = "/workspace"
	}
	cont, err := dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			AttachStderr:    true,
			AttachStdout:    true,
			AttachStdin:     true,
			OpenStdin:       true,
			StdinOnce:       true,
			Tty:             false, // Important to disable this, so that the output logs are multiplexed (stdout/stderr)
			User:            f.User,
			WorkingDir:      workdir,
			Env:             []string{"KUDE=true", "KUDE_VERSION=" + pkg.GetVersion().String()},
			Image:           f.Image,
			Entrypoint:      f.Entrypoint,
			NetworkDisabled: !f.Network,
			Labels:          map[string]string{"kude": "true", "kudeVersion": pkg.GetVersion().String()},
		},
		&container.HostConfig{Binds: f.Mounts},
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("failed creating container: %w", err)
	}
	defer func() {
		if removeErr := dockerClient.ContainerRemove(ctx, cont.ID, types.ContainerRemoveOptions{}); removeErr != nil {
			f.logger.Printf("Failed removing container '%s': %v", cont.ID, removeErr)
		}
	}()

	////////////////////////////////////////////////////////////////////////////
	// START CONTAINER
	////////////////////////////////////////////////////////////////////////////
	f.logger.Printf("Starting container '%s'", cont.ID)
	if err := dockerClient.ContainerStart(ctx, cont.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed starting container: %w", err)
	}
	defer func() {
		if stopErr := dockerClient.ContainerStop(ctx, cont.ID, &containerStopTimeout); stopErr != nil {
			f.logger.Printf("Failed stopping container '%s': %v", cont.ID, stopErr)
		}
	}()

	////////////////////////////////////////////////////////////////////////////
	// PUSH GIVEN RESOURCES INTO CONTAINER stdin
	////////////////////////////////////////////////////////////////////////////
	go func() {
		stdinAttachment, attachErr := dockerClient.ContainerAttach(ctx, cont.ID, types.ContainerAttachOptions{Stdin: true, Stream: true})
		if attachErr != nil {
			f.logger.Printf("Failed attaching to container for pushing stdin: %v", attachErr)
		}
		defer stdinAttachment.Close()
		if _, pushErr := io.Copy(stdinAttachment.Conn, r); pushErr != nil {
			f.logger.Printf("Failed pushing resources to Docker container '%s': %v", cont.ID, pushErr)
		}
	}()

	////////////////////////////////////////////////////////////////////////////
	// START READING RESOURCES FROM CONTAINER stdout, AND PIPING ITS stderr
	////////////////////////////////////////////////////////////////////////////
	logs, err := dockerClient.ContainerLogs(ctx, cont.ID, types.ContainerLogsOptions{
		Follow:     true,
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "all",
	})
	if err != nil {
		return fmt.Errorf("failed attaching to container to read stdout/stderr: %w", err)
	}
	go func() {
		if _, copyErr := stdcopy.StdCopy(w, &LogWriter{Logger: f.logger}, logs); copyErr != nil {
			f.logger.Printf("Failed copying output of container '%s': %v", cont.ID, copyErr)
		}
		logs.Close()
	}()

	////////////////////////////////////////////////////////////////////////////
	// WAIT FOR CONTAINER TO EXIT
	////////////////////////////////////////////////////////////////////////////
	var done atomic.Value
	done.Store(false)
	go func() {
		time.Sleep(5 * time.Second)
		if !done.Load().(bool) {
			f.logger.Println("Waiting for container to exit...")
		}
	}()
	statusCh, errCh := dockerClient.ContainerWait(ctx, cont.ID, container.WaitConditionNotRunning)
	var exit container.ContainerWaitOKBody
	select {
	case err := <-errCh:
		done.Store(true)
		return fmt.Errorf("failed waiting for container to exit: %w", err)
	case exit = <-statusCh:
		done.Store(true)
		err := exit.Error
		if err != nil {
			return fmt.Errorf("failed waiting for container to exit: %s", err.Message)
		} else if exit.StatusCode != 0 {
			return fmt.Errorf("container failed with exit code %d", exit.StatusCode)
		} else {
			return nil
		}
	}
}
