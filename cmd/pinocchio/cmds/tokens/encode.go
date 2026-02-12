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
	_ "github.com/tiktoken-go/tokenizer"
)

type EncodeCommand struct {
	*cmds.CommandDescription
}

var _ cmds.WriterCommand = &EncodeCommand{}

func NewEncodeCommand() (*EncodeCommand, error) {
	return &EncodeCommand{
		CommandDescription: cmds.NewCommandDescription(
			"encode",
			cmds.WithShort("Encode data using a specific model and codec"),
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

type EncodeSettings struct {
	Model string `glazed:"model"`
	Codec string `glazed:"codec"`
	Input string `glazed:"input"`
}

func (cmd *EncodeCommand) RunIntoWriter(
	ctx context.Context,
	parsedLayers *values.Values,
	w io.Writer,
) error {
	s := &EncodeSettings{}
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

	// Use tokenizer to encode
	codec := getCodec(s.Model, codecStr)
	ids, _, err := codec.Encode(s.Input)
	if err != nil {
		return errors.Wrap(err, "error encoding")
	}

	var textIds []string
	for _, id := range ids {
		textIds = append(textIds, strconv.FormatUint(uint64(id), 10))
	}

	// Write the result into provided io.Writer
	_, err = w.Write([]byte(strings.Join(textIds, " ")))
	if err != nil {
		return err
	}

	return nil
}

func getDefaultEncoding(model string) (string, error) {
	codecStr := ""
	if strings.HasPrefix(model, "gpt-4") || strings.HasPrefix(model, "gpt-3.5-turbo") || strings.HasPrefix(model, "text-embedding-ada-002") {
		codecStr = "cl100k_base"
	} else if strings.HasPrefix(model, "text-davinci-002") || strings.HasPrefix(model, "text-davinci-003") {
		codecStr = "p50k_base"
	} else {
		codecStr = "r50k_base"
	}
	if codecStr == "" {
		return "", errors.Errorf("invalid model: %s", model)
	}
	return codecStr, nil
}
