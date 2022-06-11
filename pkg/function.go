package kude

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

type Function interface {
	Invoke(logger *log.Logger, pwd, cacheDir, tempDir string, r io.Reader, w io.Writer) error
}

func InvokeFunction(f Function) {
	if pwd, err := os.Getwd(); err != nil {
		log.Fatalf("failed to get current working directory: %v", err)
	} else if err := invokeFunction(pwd, log.Default(), viper.GetViper(), ConfigFileDir, ConfigFileName, DockerCacheDir, DockerTempDir, f, os.Stdin, os.Stdout); err != nil {
		log.Fatalf("function failed: %v", err)
	}
}

// TODO: refactor InvokeFunction into a FunctionExecution construct, supported for both inline and Docker runs
func invokeFunction(pwd string, logger *log.Logger, v *viper.Viper, configFileDir, configFileName, cacheDir, tempDir string, f Function, input io.Reader, output io.Writer, opts ...viper.DecoderConfigOption) error {
	logger.SetFlags(0)
	v.SetConfigType("yaml")
	v.AddConfigPath(configFileDir)
	v.SetConfigName(strings.TrimSuffix(configFileName, filepath.Ext(configFileName)))
	v.SetEnvPrefix("KUDE")
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// no-op
		} else {
			return fmt.Errorf("failed reading configuration: %w", err)
		}
	}
	if err := v.Unmarshal(&f, opts...); err != nil {
		return fmt.Errorf("unable to decode configuration: %w", err)
	} else if err := f.Invoke(logger, pwd, cacheDir, tempDir, input, output); err != nil {
		return fmt.Errorf("failed to invoke function: %w", err)
	} else {
		return nil
	}
}
