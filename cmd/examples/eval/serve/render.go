package serve

import (
	"fmt"

	"github.com/go-go-golems/pinocchio/cmd/examples/eval/eval"
)

// RenderableTestOutput is a render-friendly version of TestOutput
// with all fields properly typed for templating
type RenderableTestOutput struct {
	ConversationString string
	EntryID            string
	GoldenAnswer       []string
	Topic              string
	Age                string
	Moral              string
	LastMessage        string
}

// NewRenderableTestOutput converts a TestOutput to a RenderableTestOutput
func NewRenderableTestOutput(output eval.TestOutput) RenderableTestOutput {
	// Helper to safely get string from interface{}
	getString := func(m map[string]interface{}, key string) string {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	return RenderableTestOutput{
		ConversationString: output.ConversationString,
		EntryID:            fmt.Sprintf("%d", output.EntryID),
		GoldenAnswer:       output.GoldenAnswer,
		Topic:              getString(output.Input, "topic"),
		Age:                getString(output.Input, "age"),
		Moral:              getString(output.Input, "moral"),
		LastMessage:        output.LastMessage,
	}
}

// ConvertOutputs converts a slice of TestOutputs to RenderableTestOutputs
func ConvertOutputs(outputs []eval.TestOutput) []RenderableTestOutput {
	result := make([]RenderableTestOutput, len(outputs))
	for i, output := range outputs {
		result[i] = NewRenderableTestOutput(output)
	}
	return result
}
