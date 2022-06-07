package commands

import (
	_ "embed"
	"github.com/arikkfir/kude/pkg"
	"github.com/spf13/cobra"
	"log"
)

//go:embed version-long.txt
var versionLongDescription string

type versioner struct {
	logger *log.Logger
}

func (v *versioner) Invoke() error {
	v.logger.Println(pkg.GetVersion().String())
	return nil
}

var versionCmd = &cobra.Command{
	Use:               "version",
	DisableAutoGenTag: true,
	Short:             "Print version information",
	Long:              versionLongDescription,
	RunE: func(cmd *cobra.Command, args []string) error {
		v := versioner{logger: log.Default()}
		return v.Invoke()
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
