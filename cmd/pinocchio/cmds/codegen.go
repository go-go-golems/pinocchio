package cmds

import (
	// legacy codegen removed
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewCodegenCommand() *cobra.Command {
	ret := &cobra.Command{
		Use:   "codegen [file...]",
		Short: "A program to convert Geppetto YAML commands into Go code",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Flag("package-name").Value.String()
			_ = cmd.Flag("output-dir").Value.String()
			return errors.New("legacy codegen removed; use engine-first commands or templates")

			// unreachable
			// return nil
		},
	}

	ret.PersistentFlags().StringP("output-dir", "o", ".", "Output directory for generated code")
	ret.PersistentFlags().StringP("package-name", "p", "main", "Package name for generated code")
	return ret
}
