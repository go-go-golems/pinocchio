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

func (c *SimpleAgentCmd) RunIntoWriter(ctx context.Context, parsed *layers.ParsedLayers, w io.Writer) error {

    // 1) Event router + sink
    router, err := events.NewEventRouter()
    if err != nil {
        return errors.Wrap(err, "router")
    }
    addPrettyHandlers(router, w)
    sink := middleware.NewWatermillSink(router.Publisher, "chat")

    // 2) Engine (avoid double events: rely on context-carried sink only)
    eng, err := factory.NewEngineFromParsedLayers(parsed)
    if err != nil {
        return errors.Wrap(err, "engine")
    }

    // 3) Tools: register a simple calculator tool
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
            updated, err := toolhelpers.RunToolCallingLoop(
                runCtx, eng, conv, registry,
                toolhelpers.NewToolConfig().
                    WithMaxIterations(5).
                    WithTimeout(30*time.Second),
            )
            if err != nil {
                return err
            }
            for _, m := range updated[len(conv):] {
                if err := manager.AppendMessages(m); err != nil {
                    return err
                }
            }

            // Ensure a newline separation between turns
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


