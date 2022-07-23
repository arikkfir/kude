package root

import (
	_ "embed"
	"github.com/spf13/cobra"
)

//go:embed description.txt
var longDescription string

var Cmd = &cobra.Command{
	Use:               "kude",
	DisableAutoGenTag: true,
	Short:             "Opinionated Kubernetes Deployment Engine",
	Long:              longDescription,
	PersistentPreRunE: populateCommandFlags,
}
