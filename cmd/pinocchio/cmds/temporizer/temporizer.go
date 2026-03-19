package temporizer

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-go-golems/glazed/pkg/helpers/files"
	"github.com/spf13/cobra"
)

func NewTemporizerCommand() *cobra.Command {
	var name, filePrefix, contentPrefix, contentSuffix string

	cmd := &cobra.Command{
		Use:   "temporizer",
		Short: "Write stdin to a temporary file and print out its name",
		Run: func(cmd *cobra.Command, args []string) {
			// Garbage Collect Existing Files
			deletedFiles, err := files.GarbageCollectTemporaryFiles(os.TempDir(), "*.tmp", 10)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error in garbage collection: %v\n", err)
				os.Exit(1)
			}
			if len(deletedFiles) > 0 {
				_, _ = fmt.Fprintln(os.Stderr, "Deleted files:", deletedFiles)
			}

			// Create a temporary file
			tempFile, err := files.CreateTemporaryFile(filePrefix, name)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error creating temporary file: %v\n", err)
				os.Exit(1)
			}
			defer func() { _ = tempFile.Close() }()

			// Write prefix content if specified
			if contentPrefix != "" {
				if !strings.HasSuffix(contentPrefix, "\n") {
					contentPrefix += "\n"
				}
				if _, err := tempFile.WriteString(contentPrefix + "\n"); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Error writing prefix content: %v\n", err)
					os.Exit(1)
				}
			}

			// Copy stdin content
			if _, err := io.Copy(tempFile, os.Stdin); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error writing content: %v\n", err)
				os.Exit(1)
			}

			// Write suffix content if specified
			if contentSuffix != "" {
				if _, err := tempFile.WriteString("\n" + contentSuffix); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Error writing suffix content: %v\n", err)
					os.Exit(1)
				}
			}

			fmt.Println(tempFile.Name())
		},
	}

	cmd.PersistentFlags().StringVarP(&name, "name", "n", "default", "Name of the temporary file")
	cmd.PersistentFlags().StringVarP(&filePrefix, "file-prefix", "p", "temporizer", "Prefix for the temporary file name")
	cmd.PersistentFlags().StringVar(&contentPrefix, "prefix", "", "Content to prepend to the input")
	cmd.PersistentFlags().StringVar(&contentSuffix, "suffix", "", "Content to append to the input")

	return cmd
}
