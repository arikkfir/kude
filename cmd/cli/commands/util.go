package commands

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
)

func populateCommandFlags(cmd *cobra.Command, _ []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Define our Viper instance and read in the config file
	v := viper.New()
	v.AddConfigPath(filepath.Join(home, ".kude"))
	v.SetConfigName("kude")
	v.SetConfigType("yaml")
	v.SetEnvPrefix("KUDE")
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	// Set values of command flags which were provided only by config file or an environment variable
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them, so bind them to their equivalent
		// keys with underscores, e.g. --favorite-color to STING_FAVORITE_COLOR
		if strings.Contains(f.Name, "-") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			if err := v.BindEnv(f.Name, fmt.Sprintf("%s_%s", "KUDE", envVarSuffix)); err != nil {
				cobra.CheckErr(err)
			}
		}

		// If flag was not given a value by Cobra (as a flag) but Viper does have a value for it - use that!
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			if err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val)); err != nil {
				cobra.CheckErr(err)
			}
		}
	})

	return nil
}
