package tools

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/go-go-golems/geppetto/pkg/inference/tools"
	uhohdsl "github.com/go-go-golems/uhoh/pkg"
	uhohdoc "github.com/go-go-golems/uhoh/pkg/doc"
	"github.com/pkg/errors"
)

// Calculator tool definitions
type CalcRequest struct {
	A  float64 `json:"a" jsonschema:"required,description=First operand"`
	B  float64 `json:"b" jsonschema:"required,description=Second operand"`
	Op string  `json:"op" jsonschema:"description=Operation,default=add,enum=add,enum=sub,enum=mul,enum=div"`
}

type CalcResponse struct {
	Result float64 `json:"result"`
}

func calculatorTool(req CalcRequest) (CalcResponse, error) {
	switch strings.ToLower(req.Op) {
	case "add":
		return CalcResponse{Result: req.A + req.B}, nil
	case "sub":
		return CalcResponse{Result: req.A - req.B}, nil
	case "mul":
		return CalcResponse{Result: req.A * req.B}, nil
	case "div":
		if req.B == 0 {
			return CalcResponse{}, errors.New("division by zero")
		}
		return CalcResponse{Result: req.A / req.B}, nil
	default:
		return CalcResponse{}, errors.Errorf("unknown op: %s", req.Op)
	}
}

// RegisterCalculatorTool registers the calc tool on the given registry.
func RegisterCalculatorTool(registry *tools.InMemoryToolRegistry) error {
	calcDef, err := tools.NewToolFromFunc(
		"calc",
		"A simple calculator that computes A (op) B where op ∈ {add, sub, mul, div}",
		calculatorTool,
	)
	if err != nil {
		return errors.Wrap(err, "calc tool")
	}
	if err := registry.RegisterTool("calc", *calcDef); err != nil {
		return errors.Wrap(err, "register calc tool")
	}
	return nil
}

// Generative UI tool definitions (integrated with Bubble Tea via a request channel)
type GenerativeUIRequest struct {
	DslYAML string `json:"dsl_yaml" jsonschema:"required,description=Uhoh DSL YAML 'form' to display in the terminal and collect structured values"`
}

type GenerativeUIResponse struct {
	Values map[string]interface{} `json:"values"`
}

// ToolUIRequest allows the tool execution goroutine to ask the UI to present a form
// and wait for the result.
type ToolUIRequest struct {
	Form    *huh.Form
	Values  map[string]interface{}
	ReplyCh chan ToolUIReply
}

type ToolUIReply struct {
	Values map[string]interface{}
	Err    error
}

// RegisterGenerativeUITool wires a tool that asks the UI to render a form based on Uhoh DSL YAML.
func RegisterGenerativeUITool(registry *tools.InMemoryToolRegistry, toolReqCh chan<- ToolUIRequest) error {
	dslDoc, err := uhohdoc.GetUhohDSLDocumentation()
	if err != nil {
		// not fatal; keep description shorter
		dslDoc = ""
	}
	genDesc := "Collect structured input from the user via a terminal form using the Uhoh DSL. " +
		"Provide the YAML in the 'dsl_yaml' field. The UI will display a form and return collected values.\n\n" +
		"Uhoh DSL guide:\n" + dslDoc

	genDef, err := tools.NewToolFromFunc(
		"generative-ui",
		genDesc,
		func(req GenerativeUIRequest) (GenerativeUIResponse, error) {
			if strings.TrimSpace(req.DslYAML) == "" {
				return GenerativeUIResponse{}, errors.New("dsl_yaml is required")
			}
			// Build huh.Form as a tea.Model and send to UI
			form, vals, err := uhohdsl.BuildBubbleTeaModelFromYAML([]byte(req.DslYAML))
			if err != nil {
				return GenerativeUIResponse{}, errors.Wrap(err, "build uhoh form")
			}
			replyCh := make(chan ToolUIReply, 1)
			toolReqCh <- ToolUIRequest{Form: form, Values: vals, ReplyCh: replyCh}

			// Wait for UI to complete the form
			select {
			case rep := <-replyCh:
				if rep.Err != nil {
					return GenerativeUIResponse{}, rep.Err
				}
				return GenerativeUIResponse{Values: rep.Values}, nil
			case <-time.After(10 * time.Minute):
				return GenerativeUIResponse{}, errors.New("form timed out")
			}
		},
	)
	if err != nil {
		return errors.Wrap(err, "generative-ui tool")
	}
	if err := registry.RegisterTool("generative-ui", *genDef); err != nil {
		return errors.Wrap(err, "register generative-ui tool")
	}
	return nil
}

// Optional: pretty json reformat helper used in pretty handlers (if needed elsewhere)
func PrettyJSON(s string) string {
	var tmp interface{}
	if strings.TrimSpace(s) == "" {
		return s
	}
	if !strings.HasPrefix(strings.TrimSpace(s), "{") && !strings.HasPrefix(strings.TrimSpace(s), "[") {
		return s
	}
	if err := json.Unmarshal([]byte(s), &tmp); err != nil {
		return s
	}
	b, err := json.MarshalIndent(tmp, "", "  ")
	if err != nil {
		return s
	}
	return string(b)
}
