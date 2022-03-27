package commands

import (
	_ "embed"
	"github.com/spf13/cobra"
)

//go:embed root-long.txt
var rootLongDescription string
var RootCmd = &cobra.Command{
	Use:               "kude",
	DisableAutoGenTag: true,
	Short:             "Opinionated Kubernetes Deployment Engine",
	Long:              rootLongDescription,
	PersistentPreRunE: populateCommandFlags,
}
