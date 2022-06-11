package version

import (
	_ "embed"
	"github.com/arikkfir/kude/cmd/cli/commands/root"
	"github.com/arikkfir/kude/pkg"
	"github.com/spf13/cobra"
	"log"
)

//go:embed description.txt
var longDescription string

var versionCmd = &cobra.Command{
	Use:               "version",
	DisableAutoGenTag: true,
	Short:             "Print version information",
	Long:              longDescription,
	Run: func(cmd *cobra.Command, args []string) {
		log.Default().Println(kude.GetVersion().String())
	},
}

func init() {
	root.Cmd.AddCommand(versionCmd)
}
