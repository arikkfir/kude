package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"time"
)

// Config is the configuration for the annotate function.
type Config struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Configuration provided by the configuration file.
var config = Config{}

// transform will annotate the given YAML resources with the configured annotation.
func transform(resources []*yaml.RNode) ([]*yaml.RNode, error) {
	for i := range resources {
		resource := resources[i]
		_, err := resource.Pipe(yaml.SetAnnotation(config.Name, config.Value))
		if err != nil {
			return nil, err
		}
	}
	return resources, nil
}

func main() {

	//
	// Read the config file
	//
	if configBytes, err := ioutil.ReadFile("/etc/kude/function/config.yaml"); err != nil {
		_ = exec.Command("ls", "-l", "/etc/kude/function/config.yaml").Run()
		time.Sleep(2 * time.Second)
		panic(err)
	} else if err := yaml.Unmarshal(configBytes, &config); err != nil {
		panic(err)
	}

	//
	// Execute pipeline on provided resources
	//
	pipeline := kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: os.Stdin}},
		Filters: []kio.Filter{kio.FilterFunc(transform)},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: os.Stdout}},
	}
	if err := pipeline.Execute(); err != nil {
		panic(err)
	}
}
