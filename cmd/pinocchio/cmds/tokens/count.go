package tokens

import (
	"context"
	"fmt"
	"io"
	"strings"

	geppettotokencount "github.com/go-go-golems/geppetto/pkg/inference/tokencount"
	tokencountfactory "github.com/go-go-golems/geppetto/pkg/inference/tokencount/factory"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/pkg/errors"
)

const (
	countModeEstimate = "estimate"
	countModeAPI      = "api"
	countModeAuto     = "auto"
	defaultCountModel = "gpt-4"
)

type CountCommand struct {
	*cmds.CommandDescription
}

func NewCountCommand() (*CountCommand, error) {
	geppettoSections, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		return nil, err
	}

	return &CountCommand{
		CommandDescription: cmds.NewCommandDescription(
			"count",
			cmds.WithShort("Count data entries using a specific model and codec"),
			cmds.WithFlags(
				fields.New(
					"model",
					fields.TypeString,
					fields.WithHelp("Model used for encoding"),
					fields.WithDefault(defaultCountModel),
				),
				fields.New(
					"codec",
					fields.TypeString,
					fields.WithHelp("Codec used for encoding"),
				),
				fields.New(
					"count-mode",
					fields.TypeString,
					fields.WithHelp("How to count tokens: estimate, api, or auto"),
					fields.WithDefault(countModeEstimate),
				),
			),
			cmds.WithArguments(
				fields.New(
					"input",
					fields.TypeStringFromFiles,
					fields.WithHelp("Input file"),
				),
			),
			cmds.WithSections(geppettoSections...),
		),
	}, nil
}

type CountSettings struct {
	Model     string `glazed:"model"`
	Codec     string `glazed:"codec"`
	Input     string `glazed:"input"`
	CountMode string `glazed:"count-mode"`
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

	s.CountMode = normalizeCountMode(s.CountMode)
	switch s.CountMode {
	case countModeEstimate:
		return cc.runLocalEstimate(s, w, nil)
	case countModeAPI, countModeAuto:
	default:
		return errors.Errorf("invalid count mode %q", s.CountMode)
	}

	stepSettings, err := aisettings.NewInferenceSettingsFromParsedValues(parsedLayers)
	if err != nil {
		return err
	}
	ensureInferenceSettingsModel(stepSettings, s.Model)

	counter, err := tokencountfactory.NewFromSettings(stepSettings)
	if err != nil {
		if s.CountMode == countModeAuto {
			return cc.runLocalEstimate(s, w, err)
		}
		return err
	}

	result, err := counter.CountTurn(ctx, &turns.Turn{
		Blocks: []turns.Block{turns.NewUserTextBlock(s.Input)},
	})
	if err != nil {
		if s.CountMode == countModeAuto {
			return cc.runLocalEstimate(s, w, err)
		}
		return err
	}

	return printProviderCountResult(w, s.CountMode, result)
}

func normalizeCountMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return countModeEstimate
	}
	return mode
}

func ensureInferenceSettingsModel(stepSettings *aisettings.InferenceSettings, model string) {
	if stepSettings == nil || stepSettings.Chat == nil {
		return
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return
	}
	if stepSettings.Chat.Engine == nil || strings.TrimSpace(*stepSettings.Chat.Engine) == "" || model != defaultCountModel {
		stepSettings.Chat.Engine = &model
	}
}

func (cc *CountCommand) runLocalEstimate(s *CountSettings, w io.Writer, fallbackErr error) error {
	count, codec, err := estimateTokenCount(s)
	if err != nil {
		if fallbackErr != nil {
			return errors.Wrapf(err, "count mode %s fallback failed after provider error: %v", s.CountMode, fallbackErr)
		}
		return err
	}
	return printEstimateCountResult(w, s.CountMode, s.Model, codec, count, fallbackErr)
}

func estimateTokenCount(s *CountSettings) (int, string, error) {
	codecStr := s.Codec
	var err error
	if codecStr == "" {
		codecStr, err = getDefaultEncoding(s.Model)
		if err != nil {
			return 0, "", errors.Wrap(err, "error getting default encoding")
		}
	}

	codec := getCodec(s.Model, codecStr)
	ids, _, err := codec.Encode(s.Input)
	if err != nil {
		return 0, "", errors.Wrap(err, "error encoding input")
	}

	return len(ids), codecStr, nil
}

func printEstimateCountResult(
	w io.Writer,
	requestedMode string,
	model string,
	codec string,
	count int,
	fallbackErr error,
) error {
	if _, err := fmt.Fprintf(w, "Requested mode: %s\n", requestedMode); err != nil {
		return errors.Wrap(err, "error writing to output")
	}
	if _, err := fmt.Fprintf(w, "Count source: %s\n", geppettotokencount.SourceEstimate); err != nil {
		return errors.Wrap(err, "error writing to output")
	}
	if fallbackErr != nil {
		if _, err := fmt.Fprintf(w, "Fallback reason: %v\n", fallbackErr); err != nil {
			return errors.Wrap(err, "error writing to output")
		}
	}
	if _, err := fmt.Fprintf(w, "Model: %s\n", model); err != nil {
		return errors.Wrap(err, "error writing to output")
	}
	if _, err := fmt.Fprintf(w, "Codec: %s\n", codec); err != nil {
		return errors.Wrap(err, "error writing to output")
	}
	if _, err := fmt.Fprintf(w, "Total tokens: %d\n", count); err != nil {
		return errors.Wrap(err, "error writing to output")
	}
	return nil
}

func printProviderCountResult(
	w io.Writer,
	requestedMode string,
	result *geppettotokencount.Result,
) error {
	if result == nil {
		return errors.New("missing token count result")
	}
	if _, err := fmt.Fprintf(w, "Requested mode: %s\n", requestedMode); err != nil {
		return errors.Wrap(err, "error writing to output")
	}
	if _, err := fmt.Fprintf(w, "Count source: %s\n", result.Source); err != nil {
		return errors.Wrap(err, "error writing to output")
	}
	if _, err := fmt.Fprintf(w, "Provider: %s\n", result.Provider); err != nil {
		return errors.Wrap(err, "error writing to output")
	}
	if _, err := fmt.Fprintf(w, "Model: %s\n", result.Model); err != nil {
		return errors.Wrap(err, "error writing to output")
	}
	if result.Endpoint != "" {
		if _, err := fmt.Fprintf(w, "Endpoint: %s\n", result.Endpoint); err != nil {
			return errors.Wrap(err, "error writing to output")
		}
	}
	if _, err := fmt.Fprintf(w, "Total tokens: %d\n", result.InputTokens); err != nil {
		return errors.Wrap(err, "error writing to output")
	}
	return nil
}
