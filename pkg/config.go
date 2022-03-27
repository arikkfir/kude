package pkg

import (
	"github.com/spf13/viper"
	"log"
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
			log.Fatalf("Failed reading function configuration: %v", err)
		}
	}
}
