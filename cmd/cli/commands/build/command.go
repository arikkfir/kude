package build

import (
	_ "embed"
	"fmt"
	"github.com/arikkfir/kude/cmd/cli/commands/root"
	"github.com/spf13/cobra"
	"log"
	"os"
)

//go:embed description.txt
var longDescription string

var buildCmd = &cobra.Command{
	Use:               "build",
	SilenceUsage:      true,
	DisableAutoGenTag: true,
	Short:             "Build the Kude package in the current directory",
	Example:           `kude build`,
	Long:              longDescription,
	RunE: func(cmd *cobra.Command, args []string) error {
		pwd := cmd.Flags().Lookup("path").Value.String()
		return build(pwd, log.Default(), cmd.OutOrStdout())
	},
}

func init() {
	pwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("failed to get current working directory: %w", err))
	}
	buildCmd.Flags().StringP("path", "p", pwd, "pipeline path (defaults to current directory)")

	root.Cmd.AddCommand(buildCmd)
}
