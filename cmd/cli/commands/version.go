package commands

import (
	_ "embed"
	"github.com/arikkfir/kude/pkg"
	"github.com/spf13/cobra"
	"log"
)

//go:embed version-long.txt
var versionLongDescription string
var versionCmd = &cobra.Command{
	Use:               "version",
	DisableAutoGenTag: true,
	Short:             "Print version information",
	Long:              versionLongDescription,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println(pkg.GetVersion().String())
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
