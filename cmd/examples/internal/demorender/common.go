package demorender

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	chatstyle "github.com/go-go-golems/bobatea/pkg/timeline/chatstyle"
	base_renderers "github.com/go-go-golems/bobatea/pkg/timeline/renderers"
	"github.com/muesli/termenv"
	"github.com/rs/zerolog/log"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

type ToolCallMarkdownBuilder func(expectedToolName, actualToolName, inputRaw string) string
type ResultMarkdownBuilder func(raw string, width int) string

func RegisterBaseRenderers(r *timeline.Registry, factories ...timeline.EntityModelFactory) {
	r.RegisterModelFactory(base_renderers.NewLLMTextFactory())
	r.RegisterModelFactory(base_renderers.PlainFactory{})
	for _, factory := range factories {
		r.RegisterModelFactory(factory)
	}
	r.RegisterModelFactory(base_renderers.LogEventFactory{})
}

type ToolCallFactory struct {
	key      string
	toolName string
	renderer *glamour.TermRenderer
	build    ToolCallMarkdownBuilder
}

func NewToolCallFactory(key string, toolName string, build ToolCallMarkdownBuilder) *ToolCallFactory {
	return &ToolCallFactory{
		key:      strings.TrimSpace(key),
		toolName: toolName,
		renderer: NewGlamourRenderer(),
		build:    build,
	}
}

func (f *ToolCallFactory) Key() string  { return f.key }
func (f *ToolCallFactory) Kind() string { return "tool_call" }
func (f *ToolCallFactory) NewEntityModel(initialProps map[string]any) timeline.EntityModel {
	m := &toolCallModel{toolName: f.toolName, renderer: f.renderer, build: f.build}
	m.onProps(initialProps)
	return m
}

type toolCallModel struct {
	toolName string
	name     string
	inputRaw string
	width    int
	selected bool
	focused  bool
	style    *chatstyle.Style
	renderer *glamour.TermRenderer
	build    ToolCallMarkdownBuilder
}

func (m *toolCallModel) Init() tea.Cmd { return nil }

func (m *toolCallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case timeline.EntitySelectedMsg:
		m.selected = true
	case timeline.EntityUnselectedMsg:
		m.selected = false
		m.focused = false
	case timeline.EntityPropsUpdatedMsg:
		if v.Patch != nil {
			m.onProps(v.Patch)
		}
	case timeline.EntitySetSizeMsg:
		m.width = v.Width
		return m, nil
	case timeline.EntityFocusMsg:
		m.focused = true
	case timeline.EntityBlurMsg:
		m.focused = false
	}
	return m, nil
}

func (m *toolCallModel) View() string {
	sty := DemoChatStyle(m.style, m.selected, m.focused)
	body := "-> " + strings.TrimSpace(m.name)
	if m.build != nil {
		mdBody := m.build(m.toolName, m.name, m.inputRaw)
		if mdBody != "" {
			body += "\n\n" + mdBody
		}
	}
	return RenderMarkdownBody(sty, m.width, m.renderer, body)
}

func (m *toolCallModel) onProps(patch map[string]any) {
	if v, ok := patch["name"].(string); ok {
		m.name = v
	}
	if v, ok := patch["input"].(string); ok {
		m.inputRaw = strings.TrimSpace(v)
	}
}

type ResultFactory struct {
	key      string
	renderer *glamour.TermRenderer
	build    ResultMarkdownBuilder
}

func NewResultFactory(key string, build ResultMarkdownBuilder) *ResultFactory {
	return &ResultFactory{
		key:      strings.TrimSpace(key),
		renderer: NewGlamourRenderer(),
		build:    build,
	}
}

func (f *ResultFactory) Key() string  { return f.key }
func (f *ResultFactory) Kind() string { return "tool_call_result" }
func (f *ResultFactory) NewEntityModel(initialProps map[string]any) timeline.EntityModel {
	m := &resultModel{renderer: f.renderer, build: f.build}
	m.onProps(initialProps)
	return m
}

type resultModel struct {
	rawResult string
	md        string
	width     int
	selected  bool
	focused   bool
	style     *chatstyle.Style
	renderer  *glamour.TermRenderer
	build     ResultMarkdownBuilder
}

func (m *resultModel) Init() tea.Cmd { return nil }

func (m *resultModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case timeline.EntitySelectedMsg:
		m.selected = true
	case timeline.EntityUnselectedMsg:
		m.selected = false
		m.focused = false
	case timeline.EntityPropsUpdatedMsg:
		if v.Patch != nil {
			m.onProps(v.Patch)
		}
	case timeline.EntitySetSizeMsg:
		m.width = v.Width
		if strings.TrimSpace(m.rawResult) != "" && m.build != nil {
			m.md = m.build(m.rawResult, m.width)
		}
		return m, nil
	case timeline.EntityFocusMsg:
		m.focused = true
	case timeline.EntityBlurMsg:
		m.focused = false
	}
	return m, nil
}

func (m *resultModel) View() string {
	sty := DemoChatStyle(m.style, m.selected, m.focused)
	body := strings.TrimSpace(m.md)
	if body == "" {
		body = strings.TrimSpace(m.rawResult)
	}
	return RenderMarkdownBody(sty, m.width, m.renderer, body)
}

func (m *resultModel) onProps(patch map[string]any) {
	if v, ok := patch["result"].(string); ok {
		m.rawResult = strings.TrimSpace(v)
		if m.build != nil {
			m.md = m.build(m.rawResult, m.width)
		}
	}
}

func RenderMarkdownBody(sty lipgloss.Style, width int, renderer *glamour.TermRenderer, body string) string {
	rendered := strings.TrimSpace(body)
	if renderer != nil && rendered != "" {
		if out, err := renderer.Render(rendered + "\n"); err == nil {
			rendered = strings.TrimSpace(out)
		}
	}
	return sty.Width(MaxInt(1, width-sty.GetHorizontalPadding())).Render(rendered)
}

func DemoChatStyle(style *chatstyle.Style, selected bool, focused bool) lipgloss.Style {
	if style == nil {
		style = chatstyle.DefaultStyles()
	}
	sty := style.UnselectedMessage
	if selected {
		sty = style.SelectedMessage
	}
	if focused && !selected {
		sty = style.FocusedMessage
	}
	return sty
}

func NewGlamourRenderer() *glamour.TermRenderer {
	style := "light"
	if !stdoutIsTerminal() {
		style = "notty"
	} else if termenv.HasDarkBackground() {
		style = "dark"
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to create glamour renderer")
		return nil
	}
	return r
}

func FencedAny(v any) string {
	switch typed := v.(type) {
	case string:
		if parsed := strings.TrimSpace(typed); parsed != "" && (strings.HasPrefix(parsed, "{") || strings.HasPrefix(parsed, "[")) {
			var anyv any
			if json.Unmarshal([]byte(parsed), &anyv) == nil {
				if y, err := yaml.Marshal(anyv); err == nil {
					return "```yaml\n" + strings.TrimSpace(string(y)) + "\n```"
				}
			}
		}
		return "```text\n" + strings.TrimSpace(typed) + "\n```"
	default:
		if y, err := yaml.Marshal(typed); err == nil {
			return "```yaml\n" + strings.TrimSpace(string(y)) + "\n```"
		}
		return fmt.Sprintf("```text\n%v\n```", typed)
	}
}

func StringifyCell(v any) string {
	switch typed := v.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		b, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(b)
	}
}

func PadRight(v string, width int) string {
	diff := width - RuneLen(v)
	if diff <= 0 {
		return v
	}
	return v + strings.Repeat(" ", diff)
}

func ClampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func RuneLen(v string) int {
	return utf8.RuneCountInString(v)
}

func TruncateRunes(v string, width int) string {
	if width <= 0 || RuneLen(v) <= width {
		return v
	}
	if width <= 1 {
		return "…"
	}
	rs := []rune(v)
	return string(rs[:width-1]) + "…"
}

func stdoutIsTerminal() bool {
	fd := os.Stdout.Fd()
	if fd > uintptr(math.MaxInt) {
		return false
	}
	return term.IsTerminal(int(fd))
}
