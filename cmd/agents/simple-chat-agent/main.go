package main

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    "github.com/ThreeDotsLabs/watermill/message"
    tea "github.com/charmbracelet/bubbletea"
    bspinner "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/bubbles/viewport"
    "github.com/charmbracelet/lipgloss"
    "github.com/go-go-golems/geppetto/pkg/conversation"
    "github.com/go-go-golems/geppetto/pkg/conversation/builder"
    "github.com/go-go-golems/geppetto/pkg/events"
    "github.com/go-go-golems/geppetto/pkg/inference/engine"
    "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
    "github.com/go-go-golems/geppetto/pkg/inference/middleware"
    "github.com/go-go-golems/geppetto/pkg/inference/toolhelpers"
    "github.com/go-go-golems/geppetto/pkg/inference/tools"
    geppettolayers "github.com/go-go-golems/geppetto/pkg/layers"
    uhoh "github.com/go-go-golems/uhoh/pkg"
    uhohdoc "github.com/go-go-golems/uhoh/pkg/doc"
    "github.com/go-go-golems/glazed/pkg/cli"
    "github.com/go-go-golems/glazed/pkg/cmds"
    "github.com/go-go-golems/glazed/pkg/cmds/layers"
    "github.com/go-go-golems/glazed/pkg/cmds/logging"
    "github.com/go-go-golems/glazed/pkg/help"
    help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
    clay "github.com/go-go-golems/clay/pkg"
    "github.com/pkg/errors"
    "github.com/rs/zerolog/log"
    "github.com/spf13/cobra"
    "golang.org/x/sync/errgroup"
    "gopkg.in/yaml.v3"
)

type SimpleAgentCmd struct{ *cmds.CommandDescription }

func NewSimpleAgentCmd() (*SimpleAgentCmd, error) {
    geLayers, err := geppettolayers.CreateGeppettoLayers()
    if err != nil {
        return nil, err
    }

    desc := cmds.NewCommandDescription(
        "simple-chat-agent",
        cmds.WithShort("Simple streaming chat agent with a calculator tool and a tiny REPL"),
        cmds.WithLayersList(geLayers...),
    )
    return &SimpleAgentCmd{CommandDescription: desc}, nil
}

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

// Generative UI tool definitions
type GenerativeUIRequest struct {
    DslYAML string `json:"dsl_yaml" jsonschema:"required,description=Uhoh DSL YAML 'form' to display in the terminal and collect structured values"`
}

type GenerativeUIResponse struct {
    Values map[string]interface{} `json:"values"`
}

func generativeUITool(req GenerativeUIRequest) (GenerativeUIResponse, error) {
    if strings.TrimSpace(req.DslYAML) == "" {
        return GenerativeUIResponse{}, errors.New("dsl_yaml is required")
    }

    var f uhoh.Form
    if err := yaml.Unmarshal([]byte(req.DslYAML), &f); err != nil {
        return GenerativeUIResponse{}, errors.Wrap(err, "unmarshal uhoh DSL YAML")
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    values, err := f.Run(ctx)
    if err != nil {
        return GenerativeUIResponse{}, errors.Wrap(err, "run uhoh form")
    }

    return GenerativeUIResponse{Values: values}, nil
}

// Lipgloss styles for pretty output
var (
    headerStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
    subHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
    toolNameStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
    jsonStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
    deltaStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
    finalStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("118"))
    errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)

// Pretty printer handler for chat events
func addPrettyHandlers(router *events.EventRouter, w io.Writer) {
    router.AddHandler("pretty", "chat", func(msg *message.Message) error {
        defer msg.Ack()
        e, err := events.NewEventFromJson(msg.Payload)
        if err != nil {
            return err
        }

        switch ev := e.(type) {
        case *events.EventPartialCompletionStart:
            fmt.Fprintln(w, headerStyle.Render("— Inference started —"))
        case *events.EventPartialCompletion:
            if ev.Delta != "" {
                fmt.Fprint(w, deltaStyle.Render(ev.Delta))
            }
        case *events.EventFinal:
            if ev.Text != "" {
                fmt.Fprintln(w, "")
                fmt.Fprintln(w, finalStyle.Render("— Inference finished —"))
            }
        case *events.EventToolCall:
            inputJSON := ev.ToolCall.Input
            if s := strings.TrimSpace(inputJSON); strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
                var tmp interface{}
                if err := json.Unmarshal([]byte(inputJSON), &tmp); err == nil {
                    if b, err := json.MarshalIndent(tmp, "", "  "); err == nil {
                        inputJSON = string(b)
                    }
                }
            }
            block := []string{
                subHeaderStyle.Render("Tool Call:"),
                toolNameStyle.Render(fmt.Sprintf("Name: %s", ev.ToolCall.Name)),
                jsonStyle.Render(fmt.Sprintf("ID: %s", ev.ToolCall.ID)),
                jsonStyle.Render(inputJSON),
            }
            fmt.Fprintln(w, strings.Join(block, "\n"))
        case *events.EventToolCallExecute:
            inputJSON := ev.ToolCall.Input
            if s := strings.TrimSpace(inputJSON); strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
                var tmp interface{}
                if err := json.Unmarshal([]byte(inputJSON), &tmp); err == nil {
                    if b, err := json.MarshalIndent(tmp, "", "  "); err == nil {
                        inputJSON = string(b)
                    }
                }
            }
            block := []string{
                subHeaderStyle.Render("Tool Execute:"),
                toolNameStyle.Render(fmt.Sprintf("Name: %s", ev.ToolCall.Name)),
                jsonStyle.Render(fmt.Sprintf("ID: %s", ev.ToolCall.ID)),
                jsonStyle.Render(inputJSON),
            }
            fmt.Fprintln(w, strings.Join(block, "\n"))
        case *events.EventToolResult:
            resultJSON := ev.ToolResult.Result
            if s := strings.TrimSpace(resultJSON); strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
                var tmp interface{}
                if err := json.Unmarshal([]byte(resultJSON), &tmp); err == nil {
                    if b, err := json.MarshalIndent(tmp, "", "  "); err == nil {
                        resultJSON = string(b)
                    }
                }
            }
            block := []string{
                subHeaderStyle.Render("Tool Result:"),
                toolNameStyle.Render(fmt.Sprintf("ID: %s", ev.ToolResult.ID)),
                jsonStyle.Render(resultJSON),
            }
            fmt.Fprintln(w, strings.Join(block, "\n"))
        case *events.EventToolCallExecutionResult:
            resultJSON := ev.ToolResult.Result
            if s := strings.TrimSpace(resultJSON); strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
                var tmp interface{}
                if err := json.Unmarshal([]byte(resultJSON), &tmp); err == nil {
                    if b, err := json.MarshalIndent(tmp, "", "  "); err == nil {
                        resultJSON = string(b)
                    }
                }
            }
            block := []string{
                subHeaderStyle.Render("Tool Exec Result:"),
                toolNameStyle.Render(fmt.Sprintf("ID: %s", ev.ToolResult.ID)),
                jsonStyle.Render(resultJSON),
            }
            fmt.Fprintln(w, strings.Join(block, "\n"))
        case *events.EventError:
            fmt.Fprintln(w, errorStyle.Render("Error: ")+ev.ErrorString)
        case *events.EventInterrupt:
            fmt.Fprintln(w, errorStyle.Render("Interrupted"))
        }
        return nil
    })
}

// addUIForwarder forwards all chat events into a channel consumed by the Bubble Tea model.
func addUIForwarder(router *events.EventRouter, ch chan<- interface{}) {
    router.AddHandler("ui-forwarder", "chat", func(msg *message.Message) error {
        defer msg.Ack()
        e, err := events.NewEventFromJson(msg.Payload)
        if err != nil {
            return err
        }
        select {
        case ch <- e:
        default:
            // drop if channel is full to avoid blocking
        }
        return nil
    })
}

// streamUIModel renders a spinner and a streaming viewport from incoming events.
type streamUIModel struct {
    spinner     bspinner.Model
    viewport    viewport.Model
    uiEvents    <-chan interface{}
    isStreaming bool
    quitWhenDone bool
    content     string
    showSpinner bool
    maxHeight   int
    termWidth   int
}

func newStreamUIModel(ch <-chan interface{}, _ io.Writer) streamUIModel {
    sp := bspinner.New()
    sp.Spinner = bspinner.Line
    sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
    vp := viewport.New(80, 3)
    // No border for minimal footprint
    vp.Style = lipgloss.NewStyle()
    return streamUIModel{
        spinner:      sp,
        viewport:     vp,
        uiEvents:     ch,
        isStreaming:  false,
        quitWhenDone: true,
        content:      "",
        showSpinner:  false,
        maxHeight:    6,
        termWidth:    80,
    }
}

func (m streamUIModel) Init() tea.Cmd {
    return tea.Batch(m.spinner.Tick, waitForUIEvent(m.uiEvents))
}

// waitForUIEvent converts a channel into a Tea command delivering one message.
func waitForUIEvent(ch <-chan interface{}) tea.Cmd {
    return func() tea.Msg {
        e, ok := <-ch
        if !ok {
            return tea.Quit
        }
        return e
    }
}

func (m streamUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch ev := msg.(type) {
    case tea.WindowSizeMsg:
        // adjust viewport height, keep a couple lines for spinner/header
        m.termWidth = ev.Width
        m.viewport.Width = ev.Width
        // Height is adapted to content elsewhere
        return m, nil
    case tea.KeyMsg:
        // ignore keyboard input; REPL handles input outside the program
        return m, nil
    case *events.EventPartialCompletionStart:
        m.isStreaming = true
        m.showSpinner = true
        return m, tea.Batch(m.spinner.Tick, waitForUIEvent(m.uiEvents))
    case *events.EventPartialCompletion:
        if ev.Delta != "" {
            m.content += ev.Delta
            m.viewport.SetContent(m.content)
            // Grow viewport height with content up to maxHeight
            lines := strings.Count(m.content, "\n") + 1
            if lines < 1 {
                lines = 1
            }
            if lines > m.maxHeight {
                lines = m.maxHeight
            }
            m.viewport.Height = lines
        }
        return m, tea.Batch(m.spinner.Tick, waitForUIEvent(m.uiEvents))
    case *events.EventToolCall:
        // Persist tool call with minimal detail
        m.content += "\n" + toolNameStyle.Render("Tool Call: ") + ev.ToolCall.Name
        if s := strings.TrimSpace(ev.ToolCall.Input); s != "" {
            m.content += "\n" + jsonStyle.Render(s)
        }
        m.viewport.SetContent(m.content)
        lines := strings.Count(m.content, "\n") + 1
        if lines > m.maxHeight { lines = m.maxHeight }
        m.viewport.Height = lines
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventToolCallExecute:
        m.content += "\n" + subHeaderStyle.Render("Executing: ") + ev.ToolCall.Name
        if s := strings.TrimSpace(ev.ToolCall.Input); s != "" {
            m.content += "\n" + jsonStyle.Render(s)
        }
        m.viewport.SetContent(m.content)
        lines := strings.Count(m.content, "\n") + 1
        if lines > m.maxHeight { lines = m.maxHeight }
        m.viewport.Height = lines
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventToolResult:
        m.content += "\n" + subHeaderStyle.Render("Tool Result:")
        if s := strings.TrimSpace(ev.ToolResult.Result); s != "" {
            m.content += "\n" + jsonStyle.Render(s)
        }
        m.viewport.SetContent(m.content)
        lines := strings.Count(m.content, "\n") + 1
        if lines > m.maxHeight { lines = m.maxHeight }
        m.viewport.Height = lines
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventToolCallExecutionResult:
        m.content += "\n" + subHeaderStyle.Render("Tool Exec Result:")
        if s := strings.TrimSpace(ev.ToolResult.Result); s != "" {
            m.content += "\n" + jsonStyle.Render(s)
        }
        m.viewport.SetContent(m.content)
        lines := strings.Count(m.content, "\n") + 1
        if lines > m.maxHeight { lines = m.maxHeight }
        m.viewport.Height = lines
        return m, waitForUIEvent(m.uiEvents)
    case *events.EventFinal:
        m.isStreaming = false
        m.showSpinner = false
        if ev.Text != "" {
            if m.content == "" || !strings.Contains(m.content, strings.TrimSpace(ev.Text)) {
                m.content += "\n" + ev.Text
                m.viewport.SetContent(m.content)
            }
        }
        if m.quitWhenDone {
            return m, tea.Quit
        }
        return m, nil
    case *events.EventError:
        m.content += "\n" + errorStyle.Render("Error: ") + ev.ErrorString
        m.viewport.SetContent(m.content)
        if m.quitWhenDone {
            return m, tea.Quit
        }
        return m, nil
    default:
        // spinner and viewport internal updates
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, tea.Batch(cmd, waitForUIEvent(m.uiEvents))
    }
}

func (m streamUIModel) View() string {
    var header string
    if m.showSpinner {
        header = headerStyle.Render("Streaming… ") + m.spinner.View()
    }
    body := m.viewport.View()
    if header != "" {
        return header + "\n" + body
    }
    return body
}

func (c *SimpleAgentCmd) RunIntoWriter(ctx context.Context, parsed *layers.ParsedLayers, w io.Writer) error {

    // 1) Event router + sink
    router, err := events.NewEventRouter()
    if err != nil {
        return errors.Wrap(err, "router")
    }
    // Forward events to a Bubble Tea UI channel (spinner + viewport)
    uiCh := make(chan interface{}, 1024)
    addUIForwarder(router, uiCh)
    sink := middleware.NewWatermillSink(router.Publisher, "chat")

    // 2) Engine (avoid double events: rely on context-carried sink only)
    eng, err := factory.NewEngineFromParsedLayers(parsed)
    if err != nil {
        return errors.Wrap(err, "engine")
    }

    // 3) Tools: register tools (calculator + generative UI)
    registry := tools.NewInMemoryToolRegistry()
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

    // Generative UI tool with embedded Uhoh DSL documentation in the description
    dslDoc, err := uhohdoc.GetUhohDSLDocumentation()
    if err != nil {
        log.Warn().Err(err).Msg("failed to load Uhoh DSL documentation for tool description")
    }
    genDesc := "Collect structured input from the user via a terminal form using the Uhoh DSL. " +
        "Provide the YAML in the 'dsl_yaml' field. The form runs and returns a JSON object of collected values.\n\n" +
        "Uhoh DSL guide:\n" + dslDoc
    genDef, err := tools.NewToolFromFunc(
        "generative-ui",
        genDesc,
        generativeUITool,
    )
    if err != nil {
        return errors.Wrap(err, "generative-ui tool")
    }
    if err := registry.RegisterTool("generative-ui", *genDef); err != nil {
        return errors.Wrap(err, "register generative-ui tool")
    }

    // Optionally configure engine tools if supported by provider
    if cfg, ok := eng.(interface{ ConfigureTools([]engine.ToolDefinition, engine.ToolConfig) }); ok {
        var defs []engine.ToolDefinition
        for _, t := range registry.ListTools() {
            defs = append(defs, engine.ToolDefinition{Name: t.Name, Description: t.Description, Parameters: t.Parameters})
        }
        cfg.ConfigureTools(defs, engine.ToolConfig{Enabled: true})
    }

    // 4) Conversation manager
    mb := builder.NewManagerBuilder().WithSystemPrompt("You are a helpful assistant. You can use tools.")
    manager, err := mb.Build()
    if err != nil {
        return errors.Wrap(err, "build conversation")
    }

    // 5) Run router and REPL in parallel
    eg, groupCtx := errgroup.WithContext(ctx)

    eg.Go(func() error { return router.Run(groupCtx) })

    eg.Go(func() error {
        <-router.Running()
        scanner := bufio.NewScanner(os.Stdin)
        fmt.Fprintln(w, headerStyle.Render("Simple Chat Agent (type :q to quit)"))
        for {
            fmt.Fprint(w, "> ")
            if !scanner.Scan() {
                return scanner.Err()
            }
            line := strings.TrimSpace(scanner.Text())
            if line == "" {
                continue
            }
            if line == ":q" || line == ":quit" || line == ":exit" {
                fmt.Fprintln(w, "Bye.")
                return nil
            }

            // Append user message and run tool-calling loop
            if err := manager.AppendMessages(conversation.NewChatMessage(conversation.RoleUser, line)); err != nil {
                return err
            }
            conv := manager.GetConversation()

            runCtx := events.WithEventSinks(groupCtx, sink)
            // Drain any leftover UI events from previous turns
            for {
                select {
                case <-uiCh:
                default:
                    goto drained
                }
            }
        drained:

            // Run Bubble Tea UI (spinner + viewport) alongside inference
            uiModel := newStreamUIModel(uiCh, w)
            pgm := tea.NewProgram(uiModel, tea.WithOutput(w))

            egTurn, turnCtx := errgroup.WithContext(runCtx)
            var updated conversation.Conversation
            var finalUIModel streamUIModel
            egTurn.Go(func() error {
                // Start router-backed streaming UI and capture final state
                m, err := pgm.Run()
                if err == nil {
                    if fm, ok := m.(streamUIModel); ok {
                        finalUIModel = fm
                    }
                }
                return err
            })
            egTurn.Go(func() error {
                var err error
                updated, err = toolhelpers.RunToolCallingLoop(
                    turnCtx, eng, conv, registry,
                    toolhelpers.NewToolConfig().
                        WithMaxIterations(5).
                        WithTimeout(30*time.Second),
                )
                return err
            })
            if err := egTurn.Wait(); err != nil {
                return err
            }
            for _, m := range updated[len(conv):] {
                if err := manager.AppendMessages(m); err != nil {
                    return err
                }
            }

            // After Bubble Tea exits, re-render the final streamed content so it remains visible
            if s := strings.TrimSpace(finalUIModel.content); s != "" {
                fmt.Fprintln(w, finalStyle.Render(s))
            }

            // Newline separation between turns
            fmt.Fprintln(w, "")
        }
    })

    if err := eg.Wait(); err != nil {
        return err
    }
    log.Info().Msg("Finished")
    return nil
}

func main() {
    root := &cobra.Command{Use: "simple-chat-agent", PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        if err := logging.InitLoggerFromViper(); err != nil {
            return err
        }
        return nil
    }}
    helpSystem := help.NewHelpSystem()
    help_cmd.SetupCobraRootCommand(helpSystem, root)

    if err := clay.InitViper("pinocchio", root); err != nil {
        cobra.CheckErr(err)
    }

    c, err := NewSimpleAgentCmd()
    cobra.CheckErr(err)
    command, err := cli.BuildCobraCommand(c, cli.WithCobraMiddlewaresFunc(geppettolayers.GetCobraCommandGeppettoMiddlewares))
    cobra.CheckErr(err)
    root.AddCommand(command)
    cobra.CheckErr(root.Execute())
}


