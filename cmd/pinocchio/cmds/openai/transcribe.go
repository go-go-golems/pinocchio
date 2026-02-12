package openai

import (
	"context"
	"os"
	"path/filepath"
	"time"

	openai_steps "github.com/go-go-golems/geppetto/pkg/steps/ai/openai"
	settings2 "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	openai_settings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	ai_types "github.com/go-go-golems/geppetto/pkg/steps/ai/types"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
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

	glazedParameterLayer, err := settings.NewGlazedSection()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	return &TranscribeCommand{
		CommandDescription: cmds.NewCommandDescription(
			"transcribe",
			cmds.WithShort("Transcribe MP3 files using OpenAI"),
			cmds.WithFlags(
				// File Input Options
				fields.New(
					"dir",
					fields.TypeString,
					fields.WithHelp("Path to the directory containing MP3 files"),
					fields.WithDefault(""),
				),
				fields.New(
					"file",
					fields.TypeString,
					fields.WithHelp("Path to the MP3 file to transcribe"),
					fields.WithDefault(""),
				),

				// Model Options
				fields.New(
					"model",
					fields.TypeString,
					fields.WithHelp("Model used for transcription"),
					fields.WithDefault(openai.Whisper1),
				),
				fields.New(
					"prompt",
					fields.TypeString,
					fields.WithHelp("Prompt for the transcription model"),
					fields.WithDefault(""),
				),
				fields.New(
					"language",
					fields.TypeString,
					fields.WithHelp("Language for the transcription model"),
					fields.WithDefault(""),
				),
				fields.New(
					"temperature",
					fields.TypeFloat,
					fields.WithHelp("Temperature for the transcription model"),
					fields.WithDefault(0.0),
				),

				// Processing Options
				fields.New(
					"max-duration",
					fields.TypeFloat,
					fields.WithHelp("Maximum duration in seconds to process"),
					fields.WithDefault(0.0),
				),
				fields.New(
					"start-time",
					fields.TypeFloat,
					fields.WithHelp("Start processing from this timestamp (in seconds)"),
					fields.WithDefault(0.0),
				),
				fields.New(
					"quality",
					fields.TypeFloat,
					fields.WithHelp("Quality level (0.0 = fastest/lowest quality, 1.0 = slowest/highest quality)"),
					fields.WithDefault(0.5),
				),

				// Output Options
				fields.New(
					"format",
					fields.TypeChoice,
					fields.WithHelp("Output format (json, verbose-json, text, srt, vtt)"),
					fields.WithChoices("json", "verbose-json", "text", "srt", "vtt"),
					fields.WithDefault("verbose-json"),
				),
				fields.New(
					"timestamps",
					fields.TypeChoiceList,
					fields.WithHelp("Timestamp granularities to include (word, segment)"),
					fields.WithChoices("word", "segment"),
					fields.WithDefault([]string{}),
				),

				// Speaker Options
				fields.New(
					"enable-diarization",
					fields.TypeBool,
					fields.WithHelp("Enable speaker diarization"),
					fields.WithDefault(false),
				),
				fields.New(
					"min-speakers",
					fields.TypeInteger,
					fields.WithHelp("Minimum number of speakers to detect"),
					fields.WithDefault(1),
				),
				fields.New(
					"max-speakers",
					fields.TypeInteger,
					fields.WithHelp("Maximum number of speakers to detect"),
					fields.WithDefault(10),
				),
				fields.New(
					"allow-profanity",
					fields.TypeBool,
					fields.WithHelp("Allow profanity in output"),
					fields.WithDefault(false),
				),

				// Performance Options
				fields.New(
					"chunk-size",
					fields.TypeInteger,
					fields.WithHelp("Size of chunks for streaming mode (in bytes)"),
					fields.WithDefault(1024*1024),
				),
				fields.New(
					"concurrent-chunks",
					fields.TypeInteger,
					fields.WithHelp("Number of chunks to process concurrently"),
					fields.WithDefault(1),
				),

				// Error Handling Options
				fields.New(
					"max-retries",
					fields.TypeInteger,
					fields.WithHelp("Maximum number of retries for failed chunks"),
					fields.WithDefault(3),
				),
				fields.New(
					"retry-delay",
					fields.TypeInteger,
					fields.WithHelp("Delay between retries (in milliseconds)"),
					fields.WithDefault(1000),
				),
				fields.New(
					"fail-fast",
					fields.TypeBool,
					fields.WithHelp("Stop on first error"),
					fields.WithDefault(false),
				),

				// Rate Limiting Options
				fields.New(
					"requests-per-minute",
					fields.TypeInteger,
					fields.WithHelp("Maximum requests per minute"),
					fields.WithDefault(60),
				),
				fields.New(
					"min-request-gap",
					fields.TypeInteger,
					fields.WithHelp("Minimum time between requests (in milliseconds)"),
					fields.WithDefault(100),
				),
				fields.New(
					"cooldown-period",
					fields.TypeInteger,
					fields.WithHelp("Time to wait when rate limit is hit (in milliseconds)"),
					fields.WithDefault(5000),
				),
			),
			cmds.WithSections(layer, glazedParameterLayer),
		),
	}, nil
}

type TranscribeSettings struct {
	// File Input Options
	DirPath  string `glazed:"dir"`
	FilePath string `glazed:"file"`

	// Model Options
	Model       string  `glazed:"model"`
	Prompt      string  `glazed:"prompt"`
	Language    string  `glazed:"language"`
	Temperature float64 `glazed:"temperature"`

	// Processing Options
	MaxDuration float64 `glazed:"max-duration"`
	StartTime   float64 `glazed:"start-time"`
	Quality     float64 `glazed:"quality"`

	// Output Options
	Format     string   `glazed:"format"`
	Timestamps []string `glazed:"timestamps"`

	// Speaker Options
	EnableDiarization bool `glazed:"enable-diarization"`
	MinSpeakers       int  `glazed:"min-speakers"`
	MaxSpeakers       int  `glazed:"max-speakers"`
	AllowProfanity    bool `glazed:"allow-profanity"`

	// Performance Options
	ChunkSize        int `glazed:"chunk-size"`
	ConcurrentChunks int `glazed:"concurrent-chunks"`

	// Error Handling Options
	MaxRetries int  `glazed:"max-retries"`
	RetryDelay int  `glazed:"retry-delay"`
	FailFast   bool `glazed:"fail-fast"`

	// Rate Limiting Options
	RequestsPerMinute int `glazed:"requests-per-minute"`
	MinRequestGap     int `glazed:"min-request-gap"`
	CooldownPeriod    int `glazed:"cooldown-period"`
}

func (c *TranscribeCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *values.Values,
	gp middlewares.Processor,
) error {
	s := &TranscribeSettings{}
	err := parsedLayers.DecodeSectionInto(values.DefaultSlug, s)
	if err != nil {
		return err
	}

	openaiSettings := &openai_settings.Settings{}
	err = parsedLayers.DecodeSectionInto(openai_settings.OpenAiChatSlug, openaiSettings)
	if err != nil {
		return err
	}

	apiSettings := &settings2.APISettings{}
	err = parsedLayers.DecodeSectionInto(openai_settings.OpenAiChatSlug, apiSettings)
	if err != nil {
		return err
	}

	openaiKey, ok := apiSettings.APIKeys[string(ai_types.ApiTypeOpenAI)+"-api-key"]
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
