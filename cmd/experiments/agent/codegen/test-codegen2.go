package codegen

import (
	context2 "context"
	context "github.com/go-go-golems/geppetto/pkg/context"
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/steps"
	"github.com/go-go-golems/geppetto/pkg/steps/ai"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	cmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"io"
)

const testCodegenCommandPrompt = "Pretend you are a {{.pretend}}. What is the {{.what}} of {{.of}}?\n"
const testCodegenCommandSystemPrompt = ""

type TestCodegenCommand struct {
	*cmds.CommandDescription
	StepSettings *settings.StepSettings  `yaml:"-"`
	Prompt       string                  `yaml:"prompt"`
	Messages     []*conversation.Message `yaml:"messages,omitempty"`
	SystemPrompt string                  `yaml:"system-prompt"`
}

type TestCodegenCommandParameters struct {
	Pretend string   `glazed.parameter:"pretend"`
	What    string   `glazed.parameter:"what"`
	Of      string   `glazed.parameter:"of"`
	Query   []string `glazed.argument:"query"`
}

var _ context.GeppettoRunnable = (*TestCodegenCommand)(nil)

func (c *TestCodegenCommand) CreateManager(
	params *TestCodegenCommandParameters,
) (*conversation.ManagerImpl, error) {
	return conversation.CreateManager(c.SystemPrompt, c.Prompt, c.Messages, params)
}

func (c *TestCodegenCommand) CreateStep(options ...chat.StepOption) (
	chat.Step,
	error,
) {
	stepFactory := &ai.StandardStepFactory{
		Settings: c.StepSettings,
	}
	return stepFactory.NewStep(options...)
}

func (c *TestCodegenCommand) RunWithManager(
	ctx context2.Context,
	manager conversation.Manager,
) (steps.StepResult[*conversation.Message], error) {
	// instantiate step frm factory
	step, err := c.CreateStep()
	if err != nil {
		return nil, err
	}

	stepResult, err := step.Start(ctx, manager.GetConversation())
	if err != nil {
		return nil, err
	}

	return stepResult, nil
}

func (c *TestCodegenCommand) RunIntoWriter(
	ctx context2.Context,
	params *TestCodegenCommandParameters,
	w io.Writer,
) error {
	manager, err := c.CreateManager(params)
	if err != nil {
		return err
	}
	return context.RunIntoWriter(ctx, c, manager, w)
}

func (c *TestCodegenCommand) RunToString(
	ctx context2.Context,
	params *TestCodegenCommandParameters,
) (string, error) {
	manager, err := c.CreateManager(params)
	if err != nil {
		return "", err
	}
	return context.RunToString(ctx, c, manager)
}

func (c *TestCodegenCommand) RunToContextManager(
	ctx context2.Context,
	params *TestCodegenCommandParameters,
) (conversation.Manager, error) {
	manager, err := c.CreateManager(params)
	if err != nil {
		return nil, err
	}
	return context.RunToContextManager(ctx, c, manager)
}

func strAddr(v string) *interface{} {
	v_ := interface{}(v)
	return &v_
}

func NewTestCodegenCommand() (*TestCodegenCommand, error) {
	var flagDefs = []*parameters.ParameterDefinition{{
		Default: strAddr("scientist"),
		Help:    "Pretend to be a ??",
		Name:    "pretend",
		Type:    "string",
	}, {
		Default: strAddr("age"),
		Help:    "What am I asking about?",
		Name:    "what",
		Type:    "string",
	}, {
		Default: strAddr("you"),
		Help:    "Of what am I asking?",
		Name:    "of",
		Type:    "string",
	}}

	var argDefs = []*parameters.ParameterDefinition{{
		Name:     "query",
		Type:     "stringList",
		Help:     "Question to answer",
		Required: true,
	}}

	cmdDescription := cmds.NewCommandDescription(
		"test-codegen",
		cmds.WithShort("Test codegen prompt"),
		cmds.WithLong("A small test prompt"),
		cmds.WithFlags(flagDefs...),
		cmds.WithArguments(argDefs...),
	)

	return &TestCodegenCommand{
		CommandDescription: cmdDescription,
		Prompt:             testCodegenCommandPrompt,
		SystemPrompt:       testCodegenCommandSystemPrompt,
		Messages:           nil,
	}, nil
}
