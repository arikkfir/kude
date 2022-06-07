package commands

import (
	_ "embed"
	"fmt"
	"github.com/arikkfir/kude/internal"
	"github.com/spf13/cobra"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

//go:embed build-long.txt
var buildLongDescription string

type builder struct {
	inlineBuiltinFunctions bool
	path                   string
	logger                 *log.Logger
	stdout                 io.Writer
}

func (b *builder) Invoke() error {
	var pwd string
	var manifestReader io.Reader
	if stat, err := os.Stat(b.path); err != nil {
		return fmt.Errorf("failed to stat path '%s': %w", b.path, err)
	} else if stat.IsDir() {
		manifestPath := filepath.Join(b.path, "kude.yaml")
		if manifestStat, err := os.Stat(manifestPath); err != nil {
			return fmt.Errorf("failed to stat path '%s': %w", manifestPath, err)
		} else if manifestStat.IsDir() {
			return fmt.Errorf("path '%s' is a directory, expected a file", manifestPath)
		} else if r, err := os.Open(manifestPath); err != nil {
			return fmt.Errorf("failed to open package manifest at '%s': %w", manifestPath, err)
		} else {
			manifestReader = r
			pwd = filepath.Dir(manifestPath)
		}
	} else if r, err := os.Open(b.path); err != nil {
		return fmt.Errorf("failed to open package manifest at '%s': %w", b.path, err)
	} else {
		manifestReader = r
		pwd = filepath.Dir(b.path)
	}

	if p, err := internal.NewPackage(b.logger, pwd, manifestReader, b.stdout, b.inlineBuiltinFunctions); err != nil {
		return fmt.Errorf("failed to build package: %w", err)
	} else if err := p.Execute(); err != nil {
		return fmt.Errorf("failed to execute package: %w", err)
	} else {
		return nil
	}
}

var buildCmd = &cobra.Command{
	Use:               "build",
	SilenceUsage:      true,
	DisableAutoGenTag: true,
	Short:             "Build the Kude package in the current directory",
	Example:           `kude build`,
	Long:              buildLongDescription,
	RunE: func(cmd *cobra.Command, args []string) error {
		inlineBuiltinFunctions, err := strconv.ParseBool(cmd.Flags().Lookup("inlineBuiltinFunctions").Value.String())
		if err != nil {
			return fmt.Errorf("failed to parse 'inlineBuiltinFunctions' flag: %w", err)
		}
		b := builder{
			inlineBuiltinFunctions: inlineBuiltinFunctions,
			path:                   cmd.Flags().Lookup("path").Value.String(),
			logger:                 log.Default(),
			stdout:                 cmd.OutOrStdout(),
		}
		return b.Invoke()
	},
}

func init() {
	pwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("failed to get current working directory: %w", err))
	}
	buildCmd.Flags().StringP("path", "p", pwd, "package path (defaults to current directory)")
	buildCmd.Flags().Bool("inlineBuiltinFunctions", false, "use inline implementation of builtin functions")
	if err := buildCmd.Flags().MarkHidden("inlineBuiltinFunctions"); err != nil {
		panic(fmt.Errorf("failed to mark 'inlineBuiltinFunctions' flag as hidden: %w", err))
	}

	RootCmd.AddCommand(buildCmd)
}
