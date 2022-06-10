package internal

import (
	"errors"
	"fmt"
	"github.com/arikkfir/kude/pkg"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"sort"
	"strings"
)

const (
	APIVersion = "kude.kfirs.com/v1alpha1"
	Kind       = "Package"
)

var internalFunctionsMapping = map[string]func() pkg.Function{
	"ghcr.io/arikkfir/kude/functions/annotate":         func() pkg.Function { return &Annotate{} },
	"ghcr.io/arikkfir/kude/functions/create-configmap": func() pkg.Function { return &CreateConfigMap{} },
	"ghcr.io/arikkfir/kude/functions/create-namespace": func() pkg.Function { return &CreateNamespace{} },
	"ghcr.io/arikkfir/kude/functions/create-secret":    func() pkg.Function { return &CreateSecret{} },
	"ghcr.io/arikkfir/kude/functions/helm":             func() pkg.Function { return &Helm{} },
	"ghcr.io/arikkfir/kude/functions/label":            func() pkg.Function { return &Label{} },
	"ghcr.io/arikkfir/kude/functions/set-namespace":    func() pkg.Function { return &SetNamespace{} },
	"ghcr.io/arikkfir/kude/functions/yq":               func() pkg.Function { return &YQ{} },
}

func NewPackage(logger *log.Logger, dir string, r io.Reader, writer io.Writer, inlineBuiltinFunctions bool) (pkg.Package, error) {
	pwd, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	return &packageImpl{logger, pwd, r, writer, inlineBuiltinFunctions}, nil
}

type packageImpl struct {
	logger                 *log.Logger
	pwd                    string
	manifestReader         io.Reader
	writer                 io.Writer
	inlineBuiltinFunctions bool
}

func (p *packageImpl) parseManifest() (*kyaml.RNode, error) {
	yaml, err := ioutil.ReadAll(p.manifestReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	manifest, err := kyaml.Parse(string(yaml))
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	} else if manifest.GetApiVersion() != APIVersion {
		return nil, fmt.Errorf("unsupported apiVersion: '%s' (should be '%s')", manifest.GetApiVersion(), APIVersion)
	} else if manifest.GetKind() != Kind {
		return nil, fmt.Errorf("unsupported kind: '%s' (should be '%s')", manifest.GetKind(), Kind)
	}
	return manifest, nil
}

func (p *packageImpl) buildPipelineInputs(manifest *kyaml.RNode) ([]kio.Reader, error) {
	resources, err := manifest.GetSlice("resources")
	if err != nil {
		if _, ok := err.(kyaml.NoFieldError); !ok {
			return nil, fmt.Errorf("failed to get resources: %w", err)
		} else {
			resources = make([]interface{}, 0)
		}
	}
	inputs := make([]kio.Reader, 0)
	for _, url := range resources {
		inputs = append(inputs, &resourceReader{
			useInternalFunctions: p.inlineBuiltinFunctions,
			logger:               p.logger,
			pwd:                  p.pwd,
			url:                  url.(string),
		})
	}
	return inputs, nil
}

func (p *packageImpl) buildPipelineFilters(manifest *kyaml.RNode, cacheDir, tempDir string) ([]kio.Filter, error) {
	pipelineNode := manifest.Field("pipeline")
	if pipelineNode == nil {
		return nil, nil
	}

	elements, err := pipelineNode.Value.Elements()
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline node: %w", err)
	}

	filters := make([]kio.Filter, 0)
	for _, functionNode := range elements {
		name, err := functionNode.GetString("name")
		if err != nil {
			name = uuid.NewString()
		}

		image, err := functionNode.GetString("image")
		origImage := image
		if err != nil {
			return nil, fmt.Errorf("failed to get image for function '%s': %w", name, err)
		} else if image == "" {
			return nil, fmt.Errorf("empty function image encountered for function '%s'", name)
		} else if !strings.Contains(image, ":") {
			origImage = image
			image = image + ":" + strings.Join(pkg.GetVersion().Build, ".")
		} else {
			repository, _, _ := strings.Cut(image, ":")
			origImage = repository
		}

		var configYAML string
		if configNode := functionNode.Field("config"); configNode != nil {
			configYAML = configNode.Value.MustString()
		} else {
			configYAML = ""
		}
		configFile := filepath.Join(tempDir, name+".yaml")
		if err := ioutil.WriteFile(configFile, []byte(configYAML), 0644); err != nil {
			return nil, fmt.Errorf("failed to write function '%s' configuration to '%s': %w", name, configFile, err)
		}

		mounts := []string{
			cacheDir + ":/workspace/.cache",
			tempDir + ":/workspace/.temp",
			configFile + ":" + pkg.ConfigFile,
		}
		if mountsList, err := functionNode.GetSlice("mounts"); err != nil {
			if _, ok := err.(kyaml.NoFieldError); !ok {
				return nil, fmt.Errorf("failed to get mounts for function '%s': %w", name, err)
			}
		} else {
			for _, v := range mountsList {
				mount := v.(string)
				local, remote, found := strings.Cut(mount, ":")
				if local == "" {
					return nil, fmt.Errorf("invalid mount format: %s", mount)
				} else if !found {
					remote = local
				}
				if !filepath.IsAbs(local) {
					local = filepath.Join(p.pwd, local)
				}
				if _, err := os.Stat(local); errors.Is(err, os.ErrNotExist) {
					return nil, fmt.Errorf("could not find '%s'", local)
				} else if err != nil {
					return nil, fmt.Errorf("failed stat for '%s': %w", local, err)
				}
				if !filepath.IsAbs(remote) {
					remote = filepath.Join("/workspace", remote)
				}
				mounts = append(mounts, local+":"+remote)
			}
		}

		logger := ChildLogger(p.logger)

		if factory, internal := internalFunctionsMapping[origImage]; internal && p.inlineBuiltinFunctions {
			function := factory()
			if err := kyaml.NewDecoder(strings.NewReader(configYAML)).Decode(function); err != nil {
				return nil, fmt.Errorf("failed to apply configuration of function '%s' from package manifest: %w", name, err)
			} else if err := function.Configure(ChildLogger(p.logger), p.pwd, cacheDir, tempDir); err != nil {
				return nil, fmt.Errorf("failed to configure function '%s': %w", name, err)
			}
			filters = append(filters, &FunctionFilterAdapter{Logger: logger, Name: name, Target: function})
		} else {
			var entrypoint []string
			entrypointSlice, err := functionNode.GetSlice("entrypoint")
			if err != nil {
				if _, ok := err.(kyaml.NoFieldError); !ok {
					return nil, fmt.Errorf("failed to get field '%s' for function '%s': %w", "entrypoint", name, err)
				}
			} else if len(entrypointSlice) > 0 {
				entrypoint = make([]string, len(entrypointSlice))
				for _, v := range entrypointSlice {
					entrypoint = append(entrypoint, v.(string))
				}
			} else {
				entrypoint = nil
			}
			user, err := functionNode.GetString("user")
			if err != nil {
				if _, ok := err.(kyaml.NoFieldError); !ok {
					return nil, fmt.Errorf("failed to get field '%s' for function '%s': %w", "user", name, err)
				}
			}
			workdir, err := functionNode.GetString("workdir")
			if err != nil {
				if _, ok := err.(kyaml.NoFieldError); !ok {
					return nil, fmt.Errorf("failed to get field '%s' for function '%s': %w", "workdir", name, err)
				}
			}
			var network bool
			if networkValue, err := functionNode.GetFieldValue("network"); err != nil {
				if _, ok := err.(kyaml.NoFieldError); !ok {
					return nil, fmt.Errorf("failed to get field '%s' for function '%s': %w", "network", name, err)
				}
			} else {
				network = networkValue.(bool)
			}
			function := DockerFunction{
				Image:      image,
				Entrypoint: entrypoint,
				User:       user,
				Workdir:    workdir,
				Network:    network,
				Mounts:     mounts,
			}
			if err := function.Configure(ChildLogger(p.logger), p.pwd, cacheDir, tempDir); err != nil {
				return nil, fmt.Errorf("failed to configure function '%s': %w", name, err)
			}
			filters = append(filters, &FunctionFilterAdapter{Logger: p.logger, Name: name, Target: &function})
		}
	}
	return filters, nil
}

func (p *packageImpl) Execute() error {
	p.logger.Printf("Executing pipeline at '%s'", p.pwd)

	cacheDir := filepath.Join(p.pwd, ".kude", "cache")
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed creating cache directory '%s': %w", cacheDir, err)
	}

	tempDir := filepath.Join(p.pwd, ".kude", "temp")
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed creating temp directory '%s': %w", tempDir, err)
	}
	defer os.RemoveAll(tempDir)

	manifest, err := p.parseManifest()
	if err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	inputs, err := p.buildPipelineInputs(manifest)
	if err != nil {
		return fmt.Errorf("failed to build pipeline inputs: %w", err)
	}

	filters, err := p.buildPipelineFilters(manifest, cacheDir, tempDir)
	if err != nil {
		return fmt.Errorf("failed to build pipeline filters: %w", err)
	}
	filters = append(
		filters,
		&ResolverFilter{},
		kio.FilterFunc(func(rns []*kyaml.RNode) ([]*kyaml.RNode, error) {
			sort.Sort(ByType(rns))
			return rns, nil
		}),
	)

	pipeline := kio.Pipeline{
		Inputs:  inputs,
		Filters: filters,
		Outputs: []kio.Writer{kio.ByteWriter{Writer: p.writer}},
	}
	if err := pipeline.Execute(); err != nil {
		return fmt.Errorf("failed to execute pipeline: %w", err)
	}
	return nil
}
