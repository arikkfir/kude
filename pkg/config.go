package pkg

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
)

const ConfigFileDir = "/etc/kude/function"
const ConfigFileName = "config.yaml"
const ConfigFile = ConfigFileDir + "/" + ConfigFileName

func Configure() {
	viper.SetConfigType("yaml")
	viper.AddConfigPath(ConfigFileDir)
	viper.SetConfigName(strings.TrimSuffix(ConfigFileName, filepath.Ext(ConfigFileName)))
	viper.SetEnvPrefix("KUDE")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// no-op
		} else {
			fmt.Fprintf(os.Stderr, "Failed reading function configuration: %s\n", err)
			os.Exit(1)
		}
	}
}
