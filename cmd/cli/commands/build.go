package commands

import (
	_ "embed"
	"fmt"
	"github.com/arikkfir/kude/internal"
	"log"
	"os"
	"sigs.k8s.io/kustomize/kyaml/kio"

	"github.com/spf13/cobra"
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

		pipeline, err := internal.NewPipeline(log.Default(), pwd, kio.ByteWriter{Writer: os.Stdout})
		if err != nil {
			return fmt.Errorf("failed to build pipeline: %w", err)
		}

		if err := pipeline.Execute(); err != nil {
			return fmt.Errorf("failed to execute pipeline: %w", err)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(buildCmd)
}
