package tokens

import (
	"context"
	"fmt"
	"io"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/pkg/errors"
)

type CountCommand struct {
	*cmds.CommandDescription
}

func NewCountCommand() (*CountCommand, error) {
	return &CountCommand{
		CommandDescription: cmds.NewCommandDescription(
			"count",
			cmds.WithShort("Count data entries using a specific model and codec"),
			cmds.WithFlags(
				fields.New(
					"model",
					fields.TypeString,
					fields.WithHelp("Model used for encoding"),
					fields.WithDefault("gpt-4"),
				),
				fields.New(
					"codec",
					fields.TypeString,
					fields.WithHelp("Codec used for encoding"),
				),
			),
			cmds.WithArguments(
				fields.New(
					"input",
					fields.TypeStringFromFiles,
					fields.WithHelp("Input file"),
				),
			),
		),
	}, nil
}

type CountSettings struct {
	Model string `glazed:"model"`
	Codec string `glazed:"codec"`
	Input string `glazed:"input"`
}

var _ cmds.WriterCommand = (*CountCommand)(nil)

func (cc *CountCommand) RunIntoWriter(
	ctx context.Context,
	parsedLayers *values.Values,
	w io.Writer,
) error {
	s := &CountSettings{}
	err := parsedLayers.DecodeSectionInto(values.DefaultSlug, s)
	if err != nil {
		return err
	}

	codecStr := s.Codec
	if s.Codec == "" {
		codecStr, err = getDefaultEncoding(s.Model)
		if err != nil {
			return errors.Wrap(err, "error getting default encoding")
		}
	}

	// Get codec based on model and codec string.
	codec := getCodec(s.Model, codecStr)

	ids, _, err := codec.Encode(s.Input)
	if err != nil {
		return errors.Wrap(err, "error encoding input")
	}

	count := len(ids)

	// Write the result to the provided writer.
	// print model and encoding
	_, err = fmt.Fprintf(w, "Model: %s\n", s.Model)
	if err != nil {
		return errors.Wrap(err, "error writing to output")
	}
	_, err = fmt.Fprintf(w, "Codec: %s\n", codecStr)
	if err != nil {
		return errors.Wrap(err, "error writing to output")
	}
	_, err = fmt.Fprintf(w, "Total tokens: %d\n", count)
	if err != nil {
		return errors.Wrap(err, "error writing to output")
	}

	return nil
}
