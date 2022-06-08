package pkg

import (
	"fmt"
	"github.com/spf13/viper"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const ConfigFileDir = "/etc/kude/function"
const ConfigFileName = "config.yaml"
const ConfigFile = ConfigFileDir + "/" + ConfigFileName
const DockerCacheDir = "/workspace/.cache"
const DockerTempDir = "/workspace/.temp"

type Package interface {
	Execute() error
}

type Function interface {
	Configure(logger *log.Logger, pwd, cacheDir, tempDir string) error
	Invoke(io.Reader, io.Writer) error
}

func InvokeFunction(f Function) {
	if err := invokeFunction(log.Default(), viper.GetViper(), ConfigFileDir, ConfigFileName, f, os.Stdin, os.Stdout); err != nil {
		log.Fatalf("function failed: %v", err)
	}
}

func invokeFunction(logger *log.Logger, v *viper.Viper, configFileDir, configFileName string, f Function, input io.Reader, output io.Writer) error {
	logger.SetFlags(0)
	v.SetConfigType("yaml")
	v.AddConfigPath(configFileDir)
	v.SetConfigName(strings.TrimSuffix(configFileName, filepath.Ext(configFileName)))
	v.SetEnvPrefix("KUDE")
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// no-op
			return nil
		} else {
			return fmt.Errorf("failed reading configuration: %w", err)
		}
	} else if err := v.Unmarshal(&f); err != nil {
		return fmt.Errorf("unable to decode configuration: %w", err)
	} else if pwd, err := os.Getwd(); err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	} else if err := f.Configure(logger, pwd, DockerCacheDir, DockerTempDir); err != nil {
		return fmt.Errorf("failed to configure function: %w", err)
	} else if err := f.Invoke(input, output); err != nil {
		return fmt.Errorf("failed to invoke function: %w", err)
	} else {
		return nil
	}
}
