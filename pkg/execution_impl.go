package kude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/arikkfir/kude/internal"
	"github.com/arikkfir/kude/internal/functions"
	"github.com/arikkfir/kyaml/pkg"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// PreviousNameAnnotationName is the name of the annotation that is used to provide the friendly resource name of
	// a resource that has been renamed for uniqueness.
	// TODO: ensure all places use this constant
	PreviousNameAnnotationName = "kude.kfirs.com/previous-name"

	// defaultInMemoryResourceCapacity is the default capacity of the in-memory resources buffer used during a pipeline
	// execution. This is used to avoid reallocating the buffer every time a new resource is added, but is required in order
	// to support resource references resolving - which requires reading all resources into memory.
	defaultInMemoryResourceCapacity = 1_000
)

func GetResourcePreviousName(r *pkg.RNode) string {
	value, err := r.GetAnnotation(PreviousNameAnnotationName)
	if err != nil {
		panic(err)
	} else {
		return value
	}
}

var (
	// containerStopTimeout is the maximum amount of time given for a container to stop, when instructed to.
	containerStopTimeout = 5 * time.Minute

	builtinFunctionsMapping = map[string]func() functions.Function{
		"ghcr.io/arikkfir/kude/functions/annotate":         func() functions.Function { return &functions.Annotate{} },
		"ghcr.io/arikkfir/kude/functions/create-configmap": func() functions.Function { return &functions.CreateConfigMap{} },
		"ghcr.io/arikkfir/kude/functions/create-namespace": func() functions.Function { return &functions.CreateNamespace{} },
		"ghcr.io/arikkfir/kude/functions/create-secret":    func() functions.Function { return &functions.CreateSecret{} },
		"ghcr.io/arikkfir/kude/functions/helm":             func() functions.Function { return &functions.Helm{} },
		"ghcr.io/arikkfir/kude/functions/label":            func() functions.Function { return &functions.Label{} },
		"ghcr.io/arikkfir/kude/functions/set-namespace":    func() functions.Function { return &functions.SetNamespace{} },
		"ghcr.io/arikkfir/kude/functions/yq":               func() functions.Function { return &functions.YQ{} },
	}
)

type executionImpl struct {
	pipeline Pipeline
	logger   *log.Logger
}

func (e *executionImpl) GetPipeline() Pipeline  { return e.pipeline }
func (e *executionImpl) GetLogger() *log.Logger { return e.logger }

func (e *executionImpl) ExecuteToWriter(ctx context.Context, w io.Writer) error {
	target := make(chan *pkg.RNode, 5000)
	exitCh := make(chan error, 1000)
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		encoder := yaml.NewEncoder(w)
		defer encoder.Close()
		for {
			rn, ok := <-target
			if ok {
				if err := encoder.Encode(rn.N); err != nil {
					exitCh <- fmt.Errorf("failed to encode node to process stdout: %w", err)
					return
				}
			} else {
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(target)
		if err := e.ExecuteToChannel(ctx, target); err != nil {
			exitCh <- err
		}
	}()

	wg.Wait()
	select {
	case err := <-exitCh:
		return err
	default:
		return nil
	}
}

func (e *executionImpl) ExecuteToChannel(ctx context.Context, target chan *pkg.RNode) error {
	timer := prometheus.NewTimer(executionsDurationHistogramMetric)
	defer timer.ObserveDuration()

	pwd := e.pipeline.GetDirectory()
	e.logger.Printf("Executing pipeline at '%s'", pwd)

	cacheDir := filepath.Join(pwd, ".kude", "cache")
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed creating cache directory '%s': %w", cacheDir, err)
	}

	tempDir := filepath.Join(pwd, ".kude", "temp")
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed creating temp directory '%s': %w", tempDir, err)
	}
	defer os.RemoveAll(tempDir)

	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("failed creating Docker client: %w", err)
	}

	// We'll wait for this wait-group to reach zero, meaning all threads finished
	wg := sync.WaitGroup{}

	// Goroutines will send errors to this channel; it's size is intentionally big to ensure goroutines do not block if/when they fail
	exitCh := make(chan error, 1000)
	defer close(exitCh)

	////////////////////////////////////////////////////////////////////////////
	// BACKGROUND READ PIPELINE INPUT RESOURCES INTO "resources"
	// ---------------------------------------------------------
	// Process each resource in the pipeline's "resources" section, recursively
	// if it is a directory, remote URL, GitHub reference, Git repository, etc.
	// Every Kubernetes resource from the processed pipeline resources will be
	// pushed into the "resources" channel, to be consumed downstream.
	////////////////////////////////////////////////////////////////////////////
	rwg := sync.WaitGroup{}
	resources := make(chan *pkg.RNode, 5000)
	for _, r := range e.pipeline.GetResources() {
		rwg.Add(1)
		go func(path string) {
			defer rwg.Done()

			timer := prometheus.NewTimer(resGenDurationHistogramMetric.WithLabelValues(path))
			defer timer.ObserveDuration()
			resGenCounterMetric.WithLabelValues(path).Inc()

			r := &resourceReader{ctx: ctx, pwd: e.GetPipeline().GetDirectory(), logger: e.GetLogger(), target: resources}
			if err := r.Read(path); err != nil {
				// TODO: add error counter
				exitCh <- fmt.Errorf("failed streaming resources found in '%s': %w", path, err)
				return
			}
		}(r)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(resources)
		rwg.Wait()
	}()

	////////////////////////////////////////////////////////////////////////////
	// INVOKE PIPELINE STEPS
	// ---------------------
	// Each step in the pipeline will be invoked in a goroutine, and will
	// consume resources from the current input channel, and push resources
	// into the next output channel. That output channel will be the input
	// channel of the next step, and so on.
	////////////////////////////////////////////////////////////////////////////
	stepInput := resources
	for _, step := range e.pipeline.GetSteps() {
		stepOutput := make(chan *pkg.RNode, 5000)
		wg.Add(1)
		go func(step Step, input chan *pkg.RNode, output chan *pkg.RNode) {
			defer wg.Done()
			defer close(output)

			timer := prometheus.NewTimer(stepDurationHistogramMetric.WithLabelValues(step.GetID(), step.GetName()))
			defer timer.ObserveDuration()
			gauge := stepGaugeMetric.WithLabelValues(step.GetID(), step.GetName())
			gauge.Inc()
			defer gauge.Dec()

			if err := e.ExecuteStep(ctx, dockerClient, cacheDir, tempDir, step, input, output); err != nil {
				exitCh <- fmt.Errorf("failed executing step '%s': %w", step.GetName(), err)
			}
		}(step, stepInput, stepOutput)
		stepInput = stepOutput // next step's input will be output of this step
	}

	////////////////////////////////////////////////////////////////////////////
	// COLLATE RENAMED RESOURCES
	// -------------------------
	// Process each resource, and collate what we call "renamed resources". These are resources that have the
	// PreviousNameAnnotationName with the original name as the value. This enables resource generators to create
	// resources with hashed names, but allow the rest of the resources to refer to those hashed resources by their
	// friendly name.
	//
	// This is a technique used by kustomize to trigger reloading of configurations or secrets when they change.
	//
	// The following goroutine will therefore process each resource, and if it contains the annotation, it will be
	// added to a mapping of such references, to be used in the next phase for resolving them.
	// This map structure is a mapping between "old" or "previous" names (i.e. ones used in references to resources)
	// to actual concrete resource names that are used in the resource declaration. Example mappings:
	//	-> "my-config": "my-config-a17bge4"
	//	-> "my-secret": "my-secret-k8fg21i"
	//
	// Additionally, the resource will be cleaned from internal annotations.
	////////////////////////////////////////////////////////////////////////////
	renamedResources := make(map[string]string)
	collatedResources := make([]*pkg.RNode, 0, defaultInMemoryResourceCapacity)
	wg.Add(1)
	go func(input chan *pkg.RNode) {
		defer wg.Done()
		for {
			rn, ok := <-input
			if ok {
				apiVersion, err := rn.GetAPIVersion()
				if err != nil {
					exitCh <- fmt.Errorf("failed getting API version for resource: %w", err)
					return
				}
				kind, err := rn.GetKind()
				if err != nil {
					exitCh <- fmt.Errorf("failed getting kind for resource: %w", err)
					return
				}
				namespace, err := rn.GetNamespace()
				if err != nil {
					exitCh <- fmt.Errorf("failed getting namespace for resource: %w", err)
					return
				}
				name, err := rn.GetName()
				if err != nil {
					exitCh <- fmt.Errorf("failed getting name for resource: %w", err)
					return
				}
				if previousName := GetResourcePreviousName(rn); previousName != "" {
					key := fmt.Sprintf("%s/%s/%s/%s", apiVersion, kind, namespace, previousName)
					renamedResources[key] = name
				}
				collatedResources = append(collatedResources, rn)
				collectedResourcesCounter.Inc()
			} else {
				break
			}
		}
		sort.Sort(ByType(collatedResources))
	}(stepInput)

	////////////////////////////////////////////////////////////////////////////
	// WAIT FOR ALL GOROUTINES TO EXIT, THEN CHECK FOR ERRORS
	////////////////////////////////////////////////////////////////////////////
	wg.Wait()
	select {
	case err := <-exitCh:
		if err != nil {
			return fmt.Errorf("pipeline error: %w", err)
		}
	default:
	}

	////////////////////////////////////////////////////////////////////////////
	// PIPE RESOURCES TO TARGET SINK
	////////////////////////////////////////////////////////////////////////////
	e.logger.Printf("Resolving references in %d resources...", len(collatedResources))
	for i, rn := range collatedResources {
		apiGroup, apiGroupVersion, err := rn.GetAPIGroupAndVersion()
		if err != nil {
			return fmt.Errorf("failed to get API group and version for resource: %w", err)
		}
		kind, err := rn.GetKind()
		if err != nil {
			return fmt.Errorf("failed to get kind for resource: %w", err)
		}
		gvk := v1.GroupVersionKind{Group: apiGroup, Version: apiGroupVersion, Kind: kind}
		if refTypes, ok := referencesCatalog[gvk]; ok {
			for _, refType := range refTypes {
				err := refType.resolve(rn, renamedResources)
				if err != nil {
					return fmt.Errorf("failed resolving references in node: %w", err)
				}
			}
		}
		resolvedResourcesCounter.Inc()
		target <- rn
		if i > 0 && i%1000 == 0 {
			e.logger.Printf("  Resolved %d resources...", i)
		}
	}
	return nil
}

func (e *executionImpl) ExecuteStep(ctx context.Context, dockerClient *client.Client, cacheDir string, tempDir string, step Step, input chan *pkg.RNode, output chan *pkg.RNode) error {
	logger := internal.NamedLogger(e.logger, step.GetID())
	logger.Printf("Executing step '%s'", step.GetName())

	// We'll wait for this wait-group to reach zero, meaning all threads finished
	wg := sync.WaitGroup{}

	// Goroutines will send errors to this channel; it's size is intentionally set to the number of goroutines we'll create
	exitCh := make(chan error, 1000)
	defer close(exitCh)

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create step input pipe: %w", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdinWriter.Close()
		encoder := yaml.NewEncoder(stdinWriter)
		encoder.SetIndent(2)
		defer encoder.Close()
		for {
			rn, ok := <-input
			if ok {
				stepInputResourcesCounter.WithLabelValues(step.GetID(), step.GetName()).Inc()
				if err := encoder.Encode(rn.N); err != nil {
					exitCh <- fmt.Errorf("failed encoding resource into container stdin: %w", err)
					return
				}
			} else {
				return
			}
		}
	}()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create output pipe: %w", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdoutWriter.Close()
		if e.pipeline.(*pipelineImpl).inlineBuiltinFunctions {
			repo, _, _ := strings.Cut(step.GetImage(), ":")
			if factory, found := builtinFunctionsMapping[repo]; found {
				if err := e.executeBuiltinFunctionInline(ctx, cacheDir, tempDir, step, logger, stdinReader, stdoutWriter, factory); err != nil {
					exitCh <- fmt.Errorf("failed to execute builtin function inline: %w", err)
				}
				return
			}
		}
		if err := e.executeContainer(ctx, dockerClient, cacheDir, tempDir, step, logger, stdinReader, stdoutWriter); err != nil {
			exitCh <- fmt.Errorf("failed running container: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		decoder := yaml.NewDecoder(stdoutReader)
		for {
			node := &yaml.Node{}
			if err := decoder.Decode(node); err != nil {
				if errors.Is(err, io.EOF) {
					return
				} else {
					exitCh <- fmt.Errorf("failed decoding YAML from container stdout: %w", err)
					return
				}
			}
			stepOutputResourcesCounter.WithLabelValues(step.GetID(), step.GetName()).Inc()
			if node.Kind == yaml.DocumentNode {
				node = node.Content[0]
			}
			if node.Kind != yaml.MappingNode {
				exitCh <- fmt.Errorf("unexpected YAML - expected object, got: %v", node.Kind)
				return
			}
			// TODO: call rn.IsValid()
			output <- &pkg.RNode{N: node}
		}
	}()

	wg.Wait()
	select {
	case err := <-exitCh:
		if err != nil {
			return fmt.Errorf("step error: %w", err)
		}
	default:
	}
	return nil
}

func (e *executionImpl) executeBuiltinFunctionInline(_ context.Context, cacheDir string, tempDir string, step Step, logger *log.Logger, stdinReader io.Reader, stdoutWriter io.Writer, factory func() functions.Function) (er error) {
	functionLogger := internal.NamedLogger(logger, "builtin")

	////////////////////////////////////////////////////////////////////////////
	// CREATE STEP CONFIGURATION FILE
	////////////////////////////////////////////////////////////////////////////
	configFileName := step.GetID() + ".yaml"
	configFile := filepath.Join(tempDir, configFileName)
	if configBytes, err := yaml.Marshal(step.GetConfig()); err != nil {
		return fmt.Errorf("failed to marshall step config: %w", err)
	} else if err := ioutil.WriteFile(configFile, configBytes, 0644); err != nil {
		return fmt.Errorf("failed to write step config to '%s': %w", configFile, err)
	}

	////////////////////////////////////////////////////////////////////////////
	// INVOKE FUNCTION
	////////////////////////////////////////////////////////////////////////////
	function := factory()
	e.pipeline.GetDirectory()
	fi := functions.FunctionInvoker{
		Function:       function,
		Pwd:            e.pipeline.GetDirectory(),
		Logger:         functionLogger,
		ConfigFileDir:  tempDir,
		ConfigFileName: configFileName,
		CacheDir:       cacheDir,
		TempDir:        tempDir,
		Viper:          viper.New(),
	}

	defer func() {
		if r := recover(); r != nil {
			if er != nil {
				er = fmt.Errorf("failed to invoke inline function: %v\noriginal error: %w", r, er)
			} else {
				er = fmt.Errorf("failed to invoke inline function: %v", r)
			}
		}
	}()
	if err := fi.Invoke(stdinReader, stdoutWriter); err != nil {
		return fmt.Errorf("failed to invoke inline function: %w", err)
	}
	return nil
}

func (e *executionImpl) executeContainer(ctx context.Context, dockerClient *client.Client, cacheDir string, tempDir string, step Step, stepLogger *log.Logger, stdinReader io.Reader, stdoutWriter io.Writer) error {
	// We'll wait for this wait-group to reach zero, meaning all threads finished
	wg := sync.WaitGroup{}

	// Goroutines will send errors to this channel; it's size is intentionally set to the number of goroutines we'll create
	exitCh := make(chan error, 1000)
	defer close(exitCh)

	pullLogger := internal.NamedLogger(stepLogger, "pull")
	containerLogger := internal.NamedLogger(stepLogger, "container")

	////////////////////////////////////////////////////////////////////////////
	// CREATE STEP CONFIGURATION FILE
	////////////////////////////////////////////////////////////////////////////
	configFile := filepath.Join(tempDir, step.GetID()+".yaml")
	if configBytes, err := yaml.Marshal(step.GetConfig()); err != nil {
		return fmt.Errorf("failed to marshall step config: %w", err)
	} else if err := ioutil.WriteFile(configFile, configBytes, 0644); err != nil {
		return fmt.Errorf("failed to write step config to '%s': %w", configFile, err)
	}

	////////////////////////////////////////////////////////////////////////////
	// BUILD MOUNTS LIST
	////////////////////////////////////////////////////////////////////////////
	mounts := []string{
		cacheDir + ":/workspace/.cache",
		tempDir + ":/workspace/.temp",
		configFile + ":" + functions.ConfigFile,
	}
	for _, mount := range step.GetMounts() {
		local, remote, found := strings.Cut(mount, ":")
		if local == "" {
			return fmt.Errorf("invalid mount format: %s", mount)
		} else if !found {
			remote = local
		}
		if !filepath.IsAbs(local) {
			local = filepath.Join(e.pipeline.GetDirectory(), local)
		}
		if _, err := os.Stat(local); errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("could not find '%s'", local)
		} else if err != nil {
			return fmt.Errorf("failed stat for '%s': %w", local, err)
		}
		if !filepath.IsAbs(remote) {
			remote = filepath.Join("/workspace", remote)
		}
		mounts = append(mounts, local+":"+remote)
	}

	////////////////////////////////////////////////////////////////////////////
	// PULL IMAGE
	////////////////////////////////////////////////////////////////////////////
	stepLogger.Printf("Pulling image '%s'", step.GetImage())
	imageListFilters := filters.NewArgs(filters.Arg("reference", step.GetImage()))
	if images, err := dockerClient.ImageList(ctx, types.ImageListOptions{Filters: imageListFilters}); err != nil {
		return fmt.Errorf("failed listing images for filter '%s': %w", step.GetImage(), err)
	} else if len(images) > 1 {
		return fmt.Errorf("found multiple matching images")
	} else if len(images) == 0 || internal.IsImageWithLatestTag(&images[0]) {
		r, err := dockerClient.ImagePull(ctx, step.GetImage(), types.ImagePullOptions{})
		if err != nil {
			return fmt.Errorf("failed pulling image: %w", err)
		}
		defer r.Close()
		s := bufio.NewScanner(r)
		for s.Scan() {
			line := s.Text()
			var pull map[string]interface{}
			if err := json.Unmarshal([]byte(line), &pull); err != nil {
				return fmt.Errorf("failed parsing image pull output: %w", err)
			}
			pullLogger.Println(pull["status"])
		}
		if s.Err() != nil {
			return fmt.Errorf("failed parsing image pull output: %w", s.Err())
		}
	}

	////////////////////////////////////////////////////////////////////////////
	// CREATE CONTAINER
	////////////////////////////////////////////////////////////////////////////
	cont, err := dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			AttachStderr:    true,
			AttachStdout:    true,
			AttachStdin:     true,
			OpenStdin:       true,
			StdinOnce:       true,
			Tty:             false, // Important to disable this, so that the output logs are multiplexed (stdout/stderr)
			User:            step.GetUser(),
			WorkingDir:      step.GetWorkdir(),
			Env:             []string{"KUDE=true", "KUDE_VERSION=" + GetVersion().String()},
			Image:           step.GetImage(),
			Entrypoint:      step.GetEntrypoint(),
			NetworkDisabled: !step.GetNetwork(),
			Labels:          map[string]string{"kude": "true", "kudeVersion": GetVersion().String()},
		},
		&container.HostConfig{Binds: mounts},
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("failed creating container: %w", err)
	}
	defer func() {
		if removeErr := dockerClient.ContainerRemove(ctx, cont.ID, types.ContainerRemoveOptions{}); removeErr != nil {
			stepLogger.Printf("Failed removing container '%s': %v", cont.ID, removeErr)
		}
	}()

	////////////////////////////////////////////////////////////////////////////
	// START CONTAINER
	////////////////////////////////////////////////////////////////////////////
	if err := dockerClient.ContainerStart(ctx, cont.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed starting container: %w", err)
	}
	defer func() {
		if stopErr := dockerClient.ContainerStop(ctx, cont.ID, &containerStopTimeout); stopErr != nil {
			stepLogger.Printf("Failed stopping container '%s': %v", cont.ID, stopErr)
		}
	}()

	////////////////////////////////////////////////////////////////////////////
	// PUSH GIVEN RESOURCES INTO CONTAINER stdin
	////////////////////////////////////////////////////////////////////////////
	wg.Add(1)
	go func() {
		defer wg.Done()
		stdinAttachment, attachErr := dockerClient.ContainerAttach(ctx, cont.ID, types.ContainerAttachOptions{Stdin: true, Stream: true})
		if attachErr != nil {
			exitCh <- fmt.Errorf("failed attaching to container stdin: %w", attachErr)
			return
		}
		defer stdinAttachment.Close()
		if _, pushErr := io.Copy(stdinAttachment.Conn, stdinReader); pushErr != nil {
			exitCh <- fmt.Errorf("failed pushing resources to stdin of container '%s': %w", cont.ID, pushErr)
			return
		}
	}()

	////////////////////////////////////////////////////////////////////////////
	// START READING RESOURCES FROM CONTAINER stdout, AND PIPING ITS stderr
	////////////////////////////////////////////////////////////////////////////
	wg.Add(1)
	go func() {
		defer wg.Done()
		logs, err := dockerClient.ContainerLogs(ctx, cont.ID, types.ContainerLogsOptions{
			Follow:     true,
			ShowStdout: true,
			ShowStderr: true,
			Tail:       "all",
		})
		if err != nil {
			exitCh <- fmt.Errorf("failed attaching to container stdout/stderr: %w", err)
			return
		}
		defer logs.Close()
		if _, copyErr := stdcopy.StdCopy(stdoutWriter, &internal.LogWriter{Logger: containerLogger}, logs); copyErr != nil {
			exitCh <- fmt.Errorf("failed piping stdout/stderr of container '%s': %w", cont.ID, err)
			return
		}
	}()

	////////////////////////////////////////////////////////////////////////////
	// WAIT FOR CONTAINER TO EXIT
	////////////////////////////////////////////////////////////////////////////
	wg.Add(1)
	go func() {
		defer wg.Done()
		statusCh, errCh := dockerClient.ContainerWait(ctx, cont.ID, container.WaitConditionNotRunning)
		var exit container.ContainerWaitOKBody
		for {
			waitCount := 0
			select {
			case err := <-errCh:
				exitCh <- fmt.Errorf("failed waiting for container to exit: %w", err)
				return
			case exit = <-statusCh:
				err := exit.Error
				if err != nil {
					exitCh <- fmt.Errorf("failed waiting for container to exit: %s", err.Message)
				} else if exit.StatusCode != 0 {
					exitCh <- fmt.Errorf("container failed with exit code %d", exit.StatusCode)
				}
				return
			default:
				waitCount++
				if waitCount == 5 {
					stepLogger.Printf("Waiting for container to exit...")
				}
				time.Sleep(time.Second)
			}
		}
	}()

	wg.Wait()
	select {
	case err := <-exitCh:
		if err != nil {
			return fmt.Errorf("container error: %w", err)
		} else {
			return nil
		}
	default:
		return nil
	}
}
