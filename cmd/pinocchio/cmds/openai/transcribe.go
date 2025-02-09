package openai

import (
	"context"
	"os"
	"path/filepath"
	"time"

	openai_steps "github.com/go-go-golems/geppetto/pkg/steps/ai/openai"
	settings2 "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	openai_settings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/sashabaranov/go-openai"
)

type TranscribeCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &TranscribeCommand{}

func NewTranscribeCommand() (*TranscribeCommand, error) {
	layer, err := openai_settings.NewParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create OpenAI parameter layer")
	}

	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	return &TranscribeCommand{
		CommandDescription: cmds.NewCommandDescription(
			"transcribe",
			cmds.WithShort("Transcribe MP3 files using OpenAI"),
			cmds.WithFlags(
				// File Input Options
				parameters.NewParameterDefinition(
					"dir",
					parameters.ParameterTypeString,
					parameters.WithHelp("Path to the directory containing MP3 files"),
					parameters.WithDefault(""),
				),
				parameters.NewParameterDefinition(
					"file",
					parameters.ParameterTypeString,
					parameters.WithHelp("Path to the MP3 file to transcribe"),
					parameters.WithDefault(""),
				),

				// Model Options
				parameters.NewParameterDefinition(
					"model",
					parameters.ParameterTypeString,
					parameters.WithHelp("Model used for transcription"),
					parameters.WithDefault(openai.Whisper1),
				),
				parameters.NewParameterDefinition(
					"prompt",
					parameters.ParameterTypeString,
					parameters.WithHelp("Prompt for the transcription model"),
					parameters.WithDefault(""),
				),
				parameters.NewParameterDefinition(
					"language",
					parameters.ParameterTypeString,
					parameters.WithHelp("Language for the transcription model"),
					parameters.WithDefault(""),
				),
				parameters.NewParameterDefinition(
					"temperature",
					parameters.ParameterTypeFloat,
					parameters.WithHelp("Temperature for the transcription model"),
					parameters.WithDefault(0.0),
				),

				// Processing Options
				parameters.NewParameterDefinition(
					"max-duration",
					parameters.ParameterTypeFloat,
					parameters.WithHelp("Maximum duration in seconds to process"),
					parameters.WithDefault(0.0),
				),
				parameters.NewParameterDefinition(
					"start-time",
					parameters.ParameterTypeFloat,
					parameters.WithHelp("Start processing from this timestamp (in seconds)"),
					parameters.WithDefault(0.0),
				),
				parameters.NewParameterDefinition(
					"quality",
					parameters.ParameterTypeFloat,
					parameters.WithHelp("Quality level (0.0 = fastest/lowest quality, 1.0 = slowest/highest quality)"),
					parameters.WithDefault(0.5),
				),

				// Output Options
				parameters.NewParameterDefinition(
					"format",
					parameters.ParameterTypeChoice,
					parameters.WithHelp("Output format (json, verbose-json, text, srt, vtt)"),
					parameters.WithChoices("json", "verbose-json", "text", "srt", "vtt"),
					parameters.WithDefault("verbose-json"),
				),
				parameters.NewParameterDefinition(
					"timestamps",
					parameters.ParameterTypeChoiceList,
					parameters.WithHelp("Timestamp granularities to include (word, segment)"),
					parameters.WithChoices("word", "segment"),
					parameters.WithDefault([]string{}),
				),

				// Speaker Options
				parameters.NewParameterDefinition(
					"enable-diarization",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Enable speaker diarization"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"min-speakers",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Minimum number of speakers to detect"),
					parameters.WithDefault(1),
				),
				parameters.NewParameterDefinition(
					"max-speakers",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Maximum number of speakers to detect"),
					parameters.WithDefault(10),
				),
				parameters.NewParameterDefinition(
					"allow-profanity",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Allow profanity in output"),
					parameters.WithDefault(false),
				),

				// Performance Options
				parameters.NewParameterDefinition(
					"chunk-size",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Size of chunks for streaming mode (in bytes)"),
					parameters.WithDefault(1024*1024),
				),
				parameters.NewParameterDefinition(
					"concurrent-chunks",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Number of chunks to process concurrently"),
					parameters.WithDefault(1),
				),

				// Error Handling Options
				parameters.NewParameterDefinition(
					"max-retries",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Maximum number of retries for failed chunks"),
					parameters.WithDefault(3),
				),
				parameters.NewParameterDefinition(
					"retry-delay",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Delay between retries (in milliseconds)"),
					parameters.WithDefault(1000),
				),
				parameters.NewParameterDefinition(
					"fail-fast",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Stop on first error"),
					parameters.WithDefault(false),
				),

				// Rate Limiting Options
				parameters.NewParameterDefinition(
					"requests-per-minute",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Maximum requests per minute"),
					parameters.WithDefault(60),
				),
				parameters.NewParameterDefinition(
					"min-request-gap",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Minimum time between requests (in milliseconds)"),
					parameters.WithDefault(100),
				),
				parameters.NewParameterDefinition(
					"cooldown-period",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Time to wait when rate limit is hit (in milliseconds)"),
					parameters.WithDefault(5000),
				),
			),
			cmds.WithLayersList(layer, glazedParameterLayer),
		),
	}, nil
}

type TranscribeSettings struct {
	// File Input Options
	DirPath  string `glazed.parameter:"dir"`
	FilePath string `glazed.parameter:"file"`

	// Model Options
	Model       string  `glazed.parameter:"model"`
	Prompt      string  `glazed.parameter:"prompt"`
	Language    string  `glazed.parameter:"language"`
	Temperature float64 `glazed.parameter:"temperature"`

	// Processing Options
	MaxDuration float64 `glazed.parameter:"max-duration"`
	StartTime   float64 `glazed.parameter:"start-time"`
	Quality     float64 `glazed.parameter:"quality"`

	// Output Options
	Format     string   `glazed.parameter:"format"`
	Timestamps []string `glazed.parameter:"timestamps"`

	// Speaker Options
	EnableDiarization bool `glazed.parameter:"enable-diarization"`
	MinSpeakers       int  `glazed.parameter:"min-speakers"`
	MaxSpeakers       int  `glazed.parameter:"max-speakers"`
	AllowProfanity    bool `glazed.parameter:"allow-profanity"`

	// Performance Options
	ChunkSize        int `glazed.parameter:"chunk-size"`
	ConcurrentChunks int `glazed.parameter:"concurrent-chunks"`

	// Error Handling Options
	MaxRetries int  `glazed.parameter:"max-retries"`
	RetryDelay int  `glazed.parameter:"retry-delay"`
	FailFast   bool `glazed.parameter:"fail-fast"`

	// Rate Limiting Options
	RequestsPerMinute int `glazed.parameter:"requests-per-minute"`
	MinRequestGap     int `glazed.parameter:"min-request-gap"`
	CooldownPeriod    int `glazed.parameter:"cooldown-period"`
}

func (c *TranscribeCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &TranscribeSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
	if err != nil {
		return err
	}

	openaiSettings := &openai_settings.Settings{}
	err = parsedLayers.InitializeStruct(openai_settings.OpenAiChatSlug, openaiSettings)
	if err != nil {
		return err
	}

	apiSettings := &settings2.APISettings{}
	err = parsedLayers.InitializeStruct(openai_settings.OpenAiChatSlug, apiSettings)
	if err != nil {
		return err
	}

	openaiKey, ok := apiSettings.APIKeys[settings2.ApiTypeOpenAI+"-api-key"]
	if !ok {
		return errors.New("no openai api key")
	}

	// Create the TranscriptionClient with options
	tc := openai_steps.NewTranscriptionClient(
		openaiKey,
		openai_steps.WithModel(s.Model),
		openai_steps.WithPrompt(s.Prompt),
		openai_steps.WithLanguage(s.Language),
		openai_steps.WithTemperature(float32(s.Temperature)),
	)

	var files []string
	if s.FilePath != "" {
		files = append(files, s.FilePath)
	}
	if s.DirPath != "" {
		files_, err := os.ReadDir(s.DirPath)
		if err != nil {
			return errors.Wrap(err, "Failed to read the directory")
		}

		for _, file := range files_ {
			files = append(files, filepath.Join(s.DirPath, file.Name()))
		}
	}

	if len(files) == 0 {
		return errors.New("No files found")
	}

	// Create transcription options
	opts := []openai_steps.TranscriptionOption{
		openai_steps.WithChunkSize(s.ChunkSize),
		openai_steps.WithConcurrentChunks(s.ConcurrentChunks),
		openai_steps.WithMaxRetries(s.MaxRetries),
		openai_steps.WithRetryDelay(time.Duration(s.RetryDelay) * time.Millisecond),
		openai_steps.WithFailFast(s.FailFast),
		openai_steps.WithRequestsPerMinute(s.RequestsPerMinute),
		openai_steps.WithMinRequestGap(time.Duration(s.MinRequestGap) * time.Millisecond),
		openai_steps.WithCooldownPeriod(time.Duration(s.CooldownPeriod) * time.Millisecond),
	}

	// Add format option
	switch s.Format {
	case "json":
		opts = append(opts, openai_steps.WithJSONFormat())
	case "verbose-json":
		opts = append(opts, openai_steps.WithVerboseJSONFormat())
	case "text":
		opts = append(opts, openai_steps.WithTextFormat())
	case "srt":
		opts = append(opts, openai_steps.WithSRTFormat())
	case "vtt":
		opts = append(opts, openai_steps.WithVTTFormat())
	}

	// Add timestamp options
	for _, ts := range s.Timestamps {
		switch ts {
		case "word":
			opts = append(opts, openai_steps.WithWordTimestamps())
		case "segment":
			opts = append(opts, openai_steps.WithSegmentTimestamps())
		}
	}

	// Add speaker options
	if s.EnableDiarization {
		opts = append(opts, openai_steps.WithSpeakerDiarization())
		opts = append(opts, openai_steps.WithSpeakerLimits(s.MinSpeakers, s.MaxSpeakers))
	}
	if s.AllowProfanity {
		opts = append(opts, openai_steps.WithProfanity())
	}

	// Process each file
	for _, file := range files {
		resp, err := tc.TranscribeFile(ctx, file, opts...)
		if err != nil {
			log.Error().Err(err).Str("file", file).Msg("Failed to transcribe")
			continue
		}

		if resp.Response == nil {
			log.Warn().Str("file", file).Msg("No response found")
			continue
		}

		// Check if word-level timestamps are requested
		hasWordTimestamps := false
		for _, ts := range s.Timestamps {
			if ts == "word" {
				hasWordTimestamps = true
				break
			}
		}

		if hasWordTimestamps && len(resp.Response.Words) > 0 {
			// Output individual words with timestamps
			for _, word := range resp.Response.Words {
				row := types.NewRow(
					types.MRP("file", resp.File),
					types.MRP("word", word.Word),
					types.MRP("start_sec", word.Start),
					types.MRP("end_sec", word.End),
				)
				if err := gp.AddRow(ctx, row); err != nil {
					return err
				}
			}
		} else if len(resp.Response.Segments) > 0 {
			// Output segments by default
			for _, segment := range resp.Response.Segments {
				row := types.NewRow(
					types.MRP("file", resp.File),
					types.MRP("start_sec", segment.Start),
					types.MRP("end_sec", segment.End),
					types.MRP("transient", segment.Transient),
					types.MRP("seek", segment.Seek),
					types.MRP("temperature", segment.Temperature),
					types.MRP("avg_logprob", segment.AvgLogprob),
					types.MRP("compression_ratio", segment.CompressionRatio),
					types.MRP("no_speech_prob", segment.NoSpeechProb),
					types.MRP("text", segment.Text),
				)
				if err := gp.AddRow(ctx, row); err != nil {
					return err
				}
			}
		} else {
			// Fallback to full text if no segments or words available
			row := types.NewRow(
				types.MRP("file", resp.File),
				types.MRP("text", resp.Response.Text),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
		}
	}

	return nil
}
