package ui

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/huh"
    boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
    toolspkg "github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/tools"
)

// OverlayModel wraps the chat model to display Huh forms requested by tools.
type OverlayModel struct {
    inner   tea.Model
    toolReq <-chan toolspkg.ToolUIRequest
    active  *huh.Form
    vals    map[string]interface{}
    reply   chan toolspkg.ToolUIReply
}

func NewOverlayModel(inner tea.Model, toolReq <-chan toolspkg.ToolUIRequest) OverlayModel {
    return OverlayModel{inner: inner, toolReq: toolReq}
}

func (m OverlayModel) Init() tea.Cmd {
    return tea.Batch(m.inner.Init(), m.waitToolReq())
}

func (m OverlayModel) waitToolReq() tea.Cmd {
    return func() tea.Msg {
        if m.toolReq == nil {
            return nil
        }
        req, ok := <-m.toolReq
        if !ok {
            return nil
        }
        return req
    }
}

func (m OverlayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch v := msg.(type) {
    case toolspkg.ToolUIRequest:
        m.active = v.Form
        m.vals = v.Values
        m.reply = v.ReplyCh
        // Blur input while form is active
        _, cmd := m.inner.Update(boba_chat.BlurInputMsg{})
        return m, tea.Batch(cmd)
    }

    if m.active != nil {
        // Update active form
        fm, cmd := m.active.Update(msg)
        if f, ok := fm.(*huh.Form); ok {
            m.active = f
        }
        if m.active.State == huh.StateCompleted && m.reply != nil {
            // reply and clear; then unblur input
            reply := m.reply
            vals := m.vals
            m.active = nil
            m.vals = nil
            m.reply = nil
            return m, tea.Batch(cmd, func() tea.Msg {
                reply <- toolspkg.ToolUIReply{Values: vals, Err: nil}
                return nil
            }, func() tea.Msg { return boba_chat.UnblurInputMsg{} }, m.waitToolReq())
        }
        return m, tea.Batch(cmd)
    }

    // Route to inner chat model
    im, cmd := m.inner.Update(msg)
    m.inner = im
    return m, tea.Batch(cmd, m.waitToolReq())
}

func (m OverlayModel) View() string {
    if m.active != nil {
        return m.active.View()
    }
    return m.inner.View()
}
