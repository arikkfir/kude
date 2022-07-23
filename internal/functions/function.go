package functions

import (
	"fmt"
	"github.com/arikkfir/kude/internal"
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

type FunctionInvoker struct {
	Function       Function
	Pwd            string
	Logger         *log.Logger
	ConfigFileDir  string
	ConfigFileName string
	CacheDir       string
	TempDir        string
	Viper          *viper.Viper
}

func (f *FunctionInvoker) Invoke(input io.Reader, output io.Writer, opts ...viper.DecoderConfigOption) error {
	v := f.Viper
	if v == nil {
		v = viper.GetViper()
	}

	pwd := f.Pwd
	if pwd == "" {
		pwd = internal.MustGetwd()
	}

	logger := f.Logger
	if logger == nil {
		logger = log.Default()
	}

	configFileDir := f.ConfigFileDir
	if configFileDir == "" {
		configFileDir = ConfigFileDir
	}

	configFileName := f.ConfigFileName
	if configFileName == "" {
		configFileName = ConfigFileName
	}

	cacheDir := f.CacheDir
	if cacheDir == "" {
		cacheDir = DockerCacheDir
	}

	tempDir := f.TempDir
	if tempDir == "" {
		tempDir = DockerTempDir
	}

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
	if err := v.Unmarshal(f.Function, opts...); err != nil {
		return fmt.Errorf("unable to decode configuration: %w", err)
	} else if err := f.Function.Invoke(logger, pwd, cacheDir, tempDir, input, output); err != nil {
		return fmt.Errorf("failed to invoke function: %w", err)
	} else {
		return nil
	}
}

func (f *FunctionInvoker) MustInvoke() {
	if err := f.Invoke(os.Stdin, os.Stdout); err != nil {
		log.Fatalf("function failed: %v", err)
	}
}
