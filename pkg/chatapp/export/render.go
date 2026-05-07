package export

import (
	"encoding/json"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Rendered struct {
	Body        []byte
	ContentType string
	Extension   string
}

func Render(value any, format Format) (Rendered, error) {
	switch NormalizeFormat(format) {
	case "", FormatJSON, FormatMinitrace:
		body, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return Rendered{}, errors.Wrap(err, "render json export")
		}
		return Rendered{Body: append(body, '\n'), ContentType: "application/json", Extension: extensionForFormat(format)}, nil
	case FormatYAML:
		body, err := yaml.Marshal(value)
		if err != nil {
			return Rendered{}, errors.Wrap(err, "render yaml export")
		}
		return Rendered{Body: body, ContentType: "application/x-yaml", Extension: ".yaml"}, nil
	case FormatMarkdown:
		return Rendered{}, errors.Wrap(ErrInvalidFormat, "markdown rendering is not implemented yet")
	default:
		return Rendered{}, ErrInvalidFormat
	}
}

func extensionForFormat(format Format) string {
	switch NormalizeFormat(format) {
	case FormatJSON, "":
		return ".json"
	case FormatMinitrace:
		return ".minitrace.json"
	case FormatYAML:
		return ".yaml"
	case FormatMarkdown:
		return ".md"
	default:
		return ".json"
	}
}
