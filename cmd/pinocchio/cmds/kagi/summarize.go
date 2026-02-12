package kagi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/pinocchio/pkg/security"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"io"
	"net/http"
)

type SummarizeCommand struct {
	*cmds.CommandDescription
}

var _ cmds.WriterCommand = &SummarizeCommand{}

type SummarizationResponse struct {
	Meta struct {
		ID   string `json:"id"`
		Node string `json:"node"`
		MS   int    `json:"ms"`
	} `json:"meta"`
	Data struct {
		Output string `json:"output"`
		Tokens int    `json:"tokens"`
	} `json:"data"`
}

type SummarizationRequest struct {
	URL            string `json:"url,omitempty"`
	Text           string `json:"text,omitempty"`
	Engine         string `json:"engine,omitempty"`
	SummaryType    string `json:"summary_type,omitempty"`
	TargetLanguage string `json:"target_language,omitempty"`
	Cache          bool   `json:"cache"`
}

type SummarizeSettings struct {
	URL            string `glazed:"url"`
	Text           string `glazed:"text"`
	Engine         string `glazed:"engine"`
	SummaryType    string `glazed:"summary_type"`
	TargetLanguage string `glazed:"target_language"`
}

func NewSummarizeCommand() (*SummarizeCommand, error) {
	return &SummarizeCommand{
		CommandDescription: cmds.NewCommandDescription(
			"summarize",
			cmds.WithShort("Summarize content"),
			cmds.WithFlags(
				fields.New(
					"url",
					fields.TypeString,
					fields.WithHelp("URL to a document to summarize"),
				),
				fields.New(
					"text",
					fields.TypeStringFromFile,
					fields.WithHelp("Text to summarize"),
					// NOTE(manuel, 2023-09-27) This exclusive with is pretty cool as an idea
					//fields.WithExclusiveWith("url"),
				),
				fields.New(
					"engine",
					fields.TypeChoice,
					fields.WithHelp("Summarization engine"),
					fields.WithChoices("agnes", "cecil", "daphne", "muriel"),
					fields.WithDefault("cecil"),
				),
				fields.New(
					"summary_type",
					fields.TypeChoice,
					fields.WithHelp("Type of summary to generate"),
					fields.WithChoices("summary", "takeaway"),
					fields.WithDefault("summary"),
				),
				fields.New(
					"target_language",
					fields.TypeString,
					fields.WithHelp("Target language for the summary"),
					fields.WithDefault("en"),
				),
			),
		),
	}, nil
}

func (c *SummarizeCommand) RunIntoWriter(
	ctx context.Context,
	parsedLayers *values.Values,
	w io.Writer,
) error {
	token := viper.GetString("kagi-api-key")
	if token == "" {
		return errors.New("no API token provided")
	}

	s := &SummarizeSettings{}
	err := parsedLayers.DecodeSectionInto(values.DefaultSlug, s)
	if err != nil {
		return errors.Wrap(err, "failed to initialize settings")
	}

	// Construct the request
	reqData := SummarizationRequest{
		URL:            s.URL,
		Text:           s.Text,
		Engine:         s.Engine,
		SummaryType:    s.SummaryType,
		TargetLanguage: s.TargetLanguage,
		Cache:          false,
	}

	bodyData, err := json.Marshal(reqData)
	if err != nil {
		return errors.Wrap(err, "failed to marshal request body")
	}

	const endpointURL = "https://kagi.com/api/v0/summarize"
	if err := security.ValidateOutboundURL(endpointURL, security.OutboundURLOptions{
		AllowHTTP: false,
	}); err != nil {
		return errors.Wrap(err, "invalid summarize endpoint URL")
	}

	req, err := http.NewRequest("POST", endpointURL, bytes.NewBuffer(bodyData))
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Authorization", "Bot "+token)
	req.Header.Set("Content-Type", "application/json")
	// #nosec G704 -- endpoint URL is validated with ValidateOutboundURL.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to send request")
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New("non-200 response received: " + string(respBody))
	}

	var response SummarizationResponse
	err = json.Unmarshal(respBody, &response)
	if err != nil {
		return errors.Wrap(err, "failed to parse response body")
	}

	// Print tokens
	fmt.Printf("Tokens: %d\n", response.Data.Tokens)
	// Print the summarization result
	fmt.Println(response.Data.Output)

	return nil
}
