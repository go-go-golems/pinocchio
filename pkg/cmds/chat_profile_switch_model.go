package cmds

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	bobatea_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/pinocchio/pkg/ui/profileswitch"
	"github.com/google/uuid"
)

type profileSwitchModel struct {
	inner tea.Model

	backend *profileswitch.Backend
	manager *profileswitch.Manager
	sink    events.EventSink

	active *huh.Form

	selectedSlug string
	options      []huh.Option[string]

	convID string
}

func newProfileSwitchModel(inner tea.Model, backend *profileswitch.Backend, manager *profileswitch.Manager, sink events.EventSink, convID string) profileSwitchModel {
	return profileSwitchModel{
		inner:   inner,
		backend: backend,
		manager: manager,
		sink:    sink,
		convID:  strings.TrimSpace(convID),
	}
}

func (m profileSwitchModel) Init() tea.Cmd { return m.inner.Init() }

func (m profileSwitchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case bobatea_chat.OpenProfilePickerMsg:
		items, err := m.manager.ListProfiles(context.Background())
		if err != nil {
			return m, systemNoticeEntityCmd(fmt.Sprintf("profile error: %s", err.Error()))
		}
		if len(items) == 0 {
			return m, systemNoticeEntityCmd("profile error: no profiles loaded")
		}

		opts := make([]huh.Option[string], 0, len(items))
		for _, it := range items {
			title := it.ProfileSlug.String()
			if strings.TrimSpace(it.DisplayName) != "" {
				title = fmt.Sprintf("%s — %s", it.ProfileSlug.String(), it.DisplayName)
			}
			if it.IsDefault {
				title = title + " (default)"
			}
			opts = append(opts, huh.NewOption(title, it.ProfileSlug.String()))
		}
		m.options = opts
		m.selectedSlug = m.backend.Current().ProfileSlug.String()

		km := huh.NewDefaultKeyMap()
		km.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc", "cancel"))

		m.active = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Switch profile").
					Options(opts...).
					Value(&m.selectedSlug),
			),
		).WithKeyMap(km)

		innerModel, cmd := m.inner.Update(bobatea_chat.BlurInputMsg{})
		m.inner = innerModel
		return m, cmd

	case tea.KeyMsg:
		if m.active != nil {
			fm, cmd := m.active.Update(v)
			if f, ok := fm.(*huh.Form); ok {
				m.active = f
			}

			if m.active != nil && m.active.State == huh.StateAborted {
				innerModel, unblurCmd := m.inner.Update(bobatea_chat.UnblurInputMsg{})
				m.inner = innerModel
				m.active = nil
				return m, tea.Batch(cmd, unblurCmd)
			}

			if m.active != nil && m.active.State == huh.StateCompleted {
				target := strings.TrimSpace(m.selectedSlug)
				from := m.backend.Current().ProfileSlug.String()

				res, err := m.backend.SwitchProfile(context.Background(), target)

				innerModel, unblurCmd := m.inner.Update(bobatea_chat.UnblurInputMsg{})
				m.inner = innerModel
				m.active = nil

				if err != nil {
					return m, tea.Batch(cmd, unblurCmd, systemNoticeEntityCmd(fmt.Sprintf("profile error: %s", err.Error())))
				}

				publishCmd := func() tea.Msg {
					_ = publishProfileSwitchedInfo(m.sink, m.convID, from, res.ProfileSlug.String(), res.RuntimeKey.String(), res.RuntimeFingerprint)
					return nil
				}

				return m, tea.Batch(
					cmd,
					unblurCmd,
					publishCmd,
					systemNoticeEntityCmd(fmt.Sprintf("switched profile: %s → %s (runtime=%s)", from, res.ProfileSlug.String(), res.RuntimeKey.String())),
				)
			}
			return m, cmd
		}
	}

	innerModel, cmd := m.inner.Update(msg)
	m.inner = innerModel
	return m, cmd
}

func (m profileSwitchModel) View() string {
	if m.active != nil {
		return m.active.View()
	}
	return m.inner.View()
}

func systemNoticeEntityCmd(text string) tea.Cmd {
	id := uuid.NewString()
	now := time.Now()
	created := func() tea.Msg {
		return timeline.UIEntityCreated{
			ID:        timeline.EntityID{LocalID: id, Kind: "llm_text"},
			Renderer:  timeline.RendererDescriptor{Kind: "llm_text"},
			Props:     map[string]any{"role": "assistant", "text": strings.TrimSpace(text), "streaming": false},
			StartedAt: now,
		}
	}
	completed := func() tea.Msg {
		return timeline.UIEntityCompleted{
			ID:     timeline.EntityID{LocalID: id, Kind: "llm_text"},
			Result: map[string]any{"text": strings.TrimSpace(text)},
		}
	}
	return tea.Batch(created, completed)
}

func publishProfileSwitchedInfo(sink events.EventSink, convID, from, to, runtimeKey, runtimeFingerprint string) error {
	if sink == nil {
		return nil
	}
	md := events.EventMetadata{
		ID: uuid.New(),
		Extra: map[string]any{
			"conversation_id":     strings.TrimSpace(convID),
			"runtime_key":         strings.TrimSpace(runtimeKey),
			"runtime_fingerprint": strings.TrimSpace(runtimeFingerprint),
			"profile.slug":        strings.TrimSpace(to),
		},
	}
	return sink.PublishEvent(events.NewInfoEvent(md, "profile-switched", map[string]any{
		"from": strings.TrimSpace(from),
		"to":   strings.TrimSpace(to),
	}))
}
