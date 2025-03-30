package main

import (
	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat/steps"
	"github.com/go-go-golems/pinocchio/pkg/ui"
	"time"
)

func main() {
	manager := conversation.NewManager(conversation.WithMessages(
		conversation.NewChatMessage(conversation.RoleSystem, "hahahahaha"),
	))

	step := &steps.EchoStep{
		TimePerCharacter: 150 * time.Millisecond,
	}

	options := []tea.ProgramOption{
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	}
	options = append(options, tea.WithAltScreen())

	backend := ui.NewStepBackend(step)
	p := tea.NewProgram(
		boba_chat.InitialModel(manager, backend),
		options...,
	)

	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
