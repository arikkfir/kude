package pkg

import (
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

type Package interface {
	Execute() error
}

type Function interface {
	Configure(logger *log.Logger, pwd, cacheDir, tempDir string) error
	Invoke(io.Reader, io.Writer) error
}

func InvokeFunction(f Function) {
	log.SetFlags(0)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(ConfigFileDir)
	viper.SetConfigName(strings.TrimSuffix(ConfigFileName, filepath.Ext(ConfigFileName)))
	viper.SetEnvPrefix("KUDE")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// no-op
		} else {
			log.Fatalf("Failed reading function configuration: %v", err)
		}
	} else if err := viper.Unmarshal(&f); err != nil {
		log.Fatalf("unable to decode configuration: %v", err)
	} else if pwd, err := os.Getwd(); err != nil {
		log.Fatalf("unable to get current working directory: %v", err)
	} else if err := f.Configure(log.Default(), pwd, "/workspace/.cache", "/workspace/.temp"); err != nil {
		log.Fatalf("failed configuring function: %v", err)
	} else if err := f.Invoke(os.Stdin, os.Stdout); err != nil {
		log.Fatalf("function execution failed: %v", err)
	}
}
