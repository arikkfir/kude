package commands

import (
	_ "embed"
	"fmt"
	"github.com/arikkfir/kude/internal"
	"github.com/spf13/cobra"
	"log"
	"os"
	"path/filepath"
)

//go:embed build-long.txt
var buildLongDescription string
var buildCmd = &cobra.Command{
	Use:               "build",
	SilenceUsage:      true,
	DisableAutoGenTag: true,
	Short:             "Build the Kude package in the current directory",
	Example:           `kude build`,
	Long:              buildLongDescription,
	RunE: func(cmd *cobra.Command, args []string) error {
		pwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		manifestPath := filepath.Join(pwd, "kude.yaml")
		manifestReader, err := os.Open(manifestPath)
		if err != nil {
			return fmt.Errorf("failed to open package manifest at '%s': %w", manifestPath, err)
		}

		if p, err := internal.NewPackage(log.Default(), pwd, manifestReader, os.Stdout, false); err != nil {
			return fmt.Errorf("failed to build package: %w", err)
		} else if err := p.Execute(); err != nil {
			return fmt.Errorf("failed to execute package: %w", err)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(buildCmd)
}
