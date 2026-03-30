package agentmode

import (
	"context"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/events/structuredsink"
)

type ExtractorConfig struct {
	ParseOptions ParseOptions `json:"parse_options,omitempty" yaml:"parse_options,omitempty"`
}

func DefaultExtractorConfig() ExtractorConfig {
	return ExtractorConfig{
		ParseOptions: DefaultParseOptions(),
	}
}

type StructuredSinkConfig struct {
	ParseOptions ParseOptions           `json:"parse_options,omitempty" yaml:"parse_options,omitempty"`
	SinkOptions  structuredsink.Options `json:"sink_options,omitempty" yaml:"sink_options,omitempty"`
}

func DefaultStructuredSinkConfig() StructuredSinkConfig {
	return StructuredSinkConfig{
		ParseOptions: DefaultParseOptions(),
		SinkOptions: structuredsink.Options{
			Malformed: structuredsink.MalformedErrorEvents,
		},
	}
}

type ModeSwitchExtractor struct {
	parseOptions ParseOptions
}

func NewModeSwitchExtractor(cfg ExtractorConfig) *ModeSwitchExtractor {
	cfg = DefaultExtractorConfig().withOverrides(cfg)
	return &ModeSwitchExtractor{parseOptions: cfg.ParseOptions}
}

func (c ExtractorConfig) withOverrides(other ExtractorConfig) ExtractorConfig {
	ret := c
	ret.ParseOptions = other.ParseOptions.withDefaults()
	return ret
}

func (e *ModeSwitchExtractor) TagPackage() string { return ModeSwitchTagPackage }
func (e *ModeSwitchExtractor) TagType() string    { return ModeSwitchTagType }
func (e *ModeSwitchExtractor) TagVersion() string { return ModeSwitchTagVersion }

func (e *ModeSwitchExtractor) NewSession(_ context.Context, _ events.EventMetadata, _ string) structuredsink.ExtractorSession {
	return &modeSwitchSession{parseOptions: e.parseOptions}
}

type modeSwitchSession struct {
	parseOptions ParseOptions
}

func (s *modeSwitchSession) OnStart(context.Context) []events.Event {
	return nil
}

func (s *modeSwitchSession) OnRaw(context.Context, []byte) []events.Event {
	return nil
}

func (s *modeSwitchSession) OnCompleted(_ context.Context, raw []byte, success bool, err error) []events.Event {
	if !success || err != nil {
		return nil
	}
	_, _ = ParseModeSwitchPayload(raw, s.parseOptions)
	return nil
}

func WrapStructuredSink(next events.EventSink, cfg StructuredSinkConfig) events.EventSink {
	cfg = DefaultStructuredSinkConfig().withOverrides(cfg)
	return structuredsink.NewFilteringSink(next, cfg.SinkOptions, NewModeSwitchExtractor(ExtractorConfig{
		ParseOptions: cfg.ParseOptions,
	}))
}

func (c StructuredSinkConfig) withOverrides(other StructuredSinkConfig) StructuredSinkConfig {
	ret := c
	ret.ParseOptions = other.ParseOptions.withDefaults()
	if other.SinkOptions.MaxCaptureBytes != 0 {
		ret.SinkOptions.MaxCaptureBytes = other.SinkOptions.MaxCaptureBytes
	}
	if other.SinkOptions.Malformed != 0 {
		ret.SinkOptions.Malformed = other.SinkOptions.Malformed
	}
	if other.SinkOptions.Debug {
		ret.SinkOptions.Debug = true
	}
	return ret
}
