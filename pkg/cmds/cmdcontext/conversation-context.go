package cmdcontext

import (
	"strings"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/glazed/pkg/helpers/templating"
	"github.com/pkg/errors"
)

type ConversationContext struct {
	SystemPrompt string
	Messages     []*conversation.Message
	Prompt       string
	Variables    map[string]interface{}
	Images       []string

	manager conversation.Manager
}

type ConversationContextOption func(*ConversationContext) error

func WithSystemPrompt(systemPrompt string) ConversationContextOption {
	return func(c *ConversationContext) error {
		c.SystemPrompt = systemPrompt
		return nil
	}
}

func WithMessages(messages []*conversation.Message) ConversationContextOption {
	return func(c *ConversationContext) error {
		c.Messages = messages
		return nil
	}
}

func WithPrompt(prompt string) ConversationContextOption {
	return func(c *ConversationContext) error {
		c.Prompt = prompt
		return nil
	}
}

func WithVariables(variables map[string]interface{}) ConversationContextOption {
	return func(c *ConversationContext) error {
		c.Variables = variables
		return nil
	}
}

func WithImages(images []string) ConversationContextOption {
	return func(c *ConversationContext) error {
		c.Images = images
		return nil
	}
}

type AutosaveSettings struct {
	Enabled  bool
	Template string
	Path     string
}

func WithAutosaveSettings(settings AutosaveSettings) ConversationContextOption {
	return func(c *ConversationContext) error {
		enabled := "no"
		if settings.Enabled {
			enabled = "yes"
		}
		c.manager = conversation.NewManager(
			conversation.WithAutosave(
				enabled,
				settings.Template,
				settings.Path,
			),
		)
		return nil
	}
}

func NewConversationContext(options ...ConversationContextOption) (*ConversationContext, error) {
	ctx := &ConversationContext{
		Variables: make(map[string]interface{}),
		manager:   conversation.NewManager(),
	}

	for _, opt := range options {
		if err := opt(ctx); err != nil {
			return nil, err
		}
	}

	err := ctx.initialize()
	if err != nil {
		return nil, err
	}

	return ctx, nil
}

func (c *ConversationContext) initialize() error {
	if c.SystemPrompt != "" {
		systemPromptTemplate, err := templating.CreateTemplate("system-prompt").Parse(c.SystemPrompt)
		if err != nil {
			return errors.Wrap(err, "failed to parse system prompt template")
		}

		var systemPromptBuffer strings.Builder
		err = systemPromptTemplate.Execute(&systemPromptBuffer, c.Variables)
		if err != nil {
			return errors.Wrap(err, "failed to execute system prompt template")
		}

		c.manager.AppendMessages(conversation.NewChatMessage(
			conversation.RoleSystem,
			systemPromptBuffer.String(),
		))
	}

	for _, message_ := range c.Messages {
		switch content := message_.Content.(type) {
		case *conversation.ChatMessageContent:
			messageTemplate, err := templating.CreateTemplate("message").Parse(content.Text)
			if err != nil {
				return errors.Wrap(err, "failed to parse message template")
			}

			var messageBuffer strings.Builder
			err = messageTemplate.Execute(&messageBuffer, c.Variables)
			if err != nil {
				return errors.Wrap(err, "failed to execute message template")
			}
			s_ := messageBuffer.String()

			c.manager.AppendMessages(conversation.NewChatMessage(
				content.Role, s_, conversation.WithTime(message_.Time)))
		}
	}

	if c.Prompt != "" {
		promptTemplate, err := templating.CreateTemplate("prompt").Parse(c.Prompt)
		if err != nil {
			return errors.Wrap(err, "failed to parse prompt template")
		}

		var promptBuffer strings.Builder
		err = promptTemplate.Execute(&promptBuffer, c.Variables)
		if err != nil {
			return errors.Wrap(err, "failed to execute prompt template")
		}

		images := []*conversation.ImageContent{}
		for _, imagePath := range c.Images {
			image, err := conversation.NewImageContentFromFile(imagePath)
			if err != nil {
				return errors.Wrap(err, "failed to create image content")
			}
			images = append(images, image)
		}

		messageContent := &conversation.ChatMessageContent{
			Role:   conversation.RoleUser,
			Text:   promptBuffer.String(),
			Images: images,
		}
		c.manager.AppendMessages(conversation.NewMessage(messageContent))
	}

	return nil
}

func (c *ConversationContext) GetManager() conversation.Manager {
	return c.manager
}

func (c *ConversationContext) AppendImageToPrompt(imagePath string) error {
	if c.Prompt == "" {
		return errors.New("cannot append image to empty prompt")
	}

	image, err := conversation.NewImageContentFromFile(imagePath)
	if err != nil {
		return errors.Wrap(err, "failed to create image content")
	}

	lastMessage := c.manager.GetConversation()[len(c.manager.GetConversation())-1]
	if content, ok := lastMessage.Content.(*conversation.ChatMessageContent); ok {
		content.Images = append(content.Images, image)
	}

	return nil
}
