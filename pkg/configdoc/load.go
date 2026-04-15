package configdoc

import (
	"bytes"
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func LoadDocument(path string) (*Document, error) {
	if err := ValidateLocalOverrideFileName(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read config document")
	}
	return DecodeDocument(data)
}

func DecodeDocument(data []byte) (*Document, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	doc := &Document{}
	if err := decoder.Decode(doc); err != nil {
		return nil, errors.Wrap(err, "decode config document")
	}
	if err := doc.NormalizeAndValidate(); err != nil {
		return nil, err
	}
	return doc, nil
}
