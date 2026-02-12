package tokens

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/pkg/errors"
)

type DecodeCommand struct {
	*cmds.CommandDescription
}

var _ cmds.WriterCommand = &DecodeCommand{}

func NewDecodeCommand() (*DecodeCommand, error) {
	return &DecodeCommand{
		CommandDescription: cmds.NewCommandDescription(
			"decode",
			cmds.WithShort("Decode data using a specific model and codec"),
			cmds.WithFlags(
				fields.New(
					"model",
					fields.TypeString,
					fields.WithHelp("Model used for encoding"),
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

type DecodeSettings struct {
	Model string `glazed:"model"`
	Codec string `glazed:"codec"`
	Input string `glazed:"input"`
}

func (d *DecodeCommand) RunIntoWriter(
	ctx context.Context,
	parsedLayers *values.Values,
	w io.Writer,
) error {
	s := &DecodeSettings{}
	err := parsedLayers.DecodeSectionInto(values.DefaultSlug, s)
	if err != nil {
		return err
	}
	// Retrieve parsed parameters from the layers.
	codecStr := s.Codec
	if codecStr == "" {
		codecStr, err = getDefaultEncoding(s.Model)
		if err != nil {
			return errors.Wrap(err, "error getting default encoding")
		}
	}

	// Get codec based on model and codec string.
	codec := getCodec(s.Model, codecStr)

	// Decode input
	var ids []uint
	for _, t := range strings.Split(s.Input, " ") {
		id, err := strconv.Atoi(t)
		if err != nil {
			return errors.Errorf("invalid token id: %s", t)
		}
		if id < 0 {
			return errors.Errorf("invalid token ID: %d (must be non-negative)", id)
		}
		ids = append(ids, uint(id))
	}

	text, err := codec.Decode(ids)
	if err != nil {
		return errors.Wrap(err, "error decoding")
	}

	// Write the result to the provided writer
	_, err = w.Write([]byte(text))
	return err
}
