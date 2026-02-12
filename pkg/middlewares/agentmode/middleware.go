package agentmode

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/events"
	rootmw "github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/steps/parse"
	"github.com/go-go-golems/geppetto/pkg/turns"
	gcompat "github.com/go-go-golems/pinocchio/pkg/geppettocompat"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// AgentMode describes a mode name with allowed tools and an optional system prompt snippet.
type AgentMode struct {
	Name         string
	AllowedTools []string
	Prompt       string
}

// Resolver resolves a mode name to its definition.
// Deprecated: Resolver and Store merged into Service in service.go
type Resolver interface {
	GetMode(ctx context.Context, name string) (*AgentMode, error)
}
type Store interface {
	GetCurrentMode(ctx context.Context, sessionID string) (string, error)
	RecordModeChange(ctx context.Context, change ModeChange) error
}

// ModeChange captures a mode transition with optional analysis text.
type ModeChange struct {
	SessionID string
	TurnID    string
	FromMode  string
	ToMode    string
	Analysis  string
	At        time.Time
}

// Config configures the behavior of the middleware.
type Config struct {
	DefaultMode string
}

func DefaultConfig() Config {
	return Config{
		DefaultMode: "default",
	}
}

func publishAgentModeSwitchEvent(ctx context.Context, meta events.EventMetadata, from string, to string, analysis string) {
	events.PublishEventToContext(ctx, events.NewAgentModeSwitchEvent(meta, from, to, analysis))
}

func sessionIDFromTurn(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	if sid, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata); err == nil && ok {
		return sid
	}
	return ""
}

// NewMiddleware returns a middleware.Middleware compatible handler.
func NewMiddleware(svc Service, cfg Config) rootmw.Middleware {
	return func(next rootmw.HandlerFunc) rootmw.HandlerFunc {
		return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
			if t == nil {
				return next(ctx, t)
			}

<<<<<<< HEAD
			sessionID := sessionIDFromTurn(t)
			log.Debug().Str("session_id", sessionID).Str("turn_id", t.ID).Msg("agentmode: middleware start")
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
			log.Debug().Str("run_id", t.RunID).Str("turn_id", t.ID).Msg("agentmode: middleware start")
=======
			runID := gcompat.TurnSessionID(t)
			log.Debug().Str("run_id", runID).Str("turn_id", t.ID).Msg("agentmode: middleware start")
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)

			// Determine current mode: from Turn.Data or Store fallback
			modeName, ok, err := turns.KeyAgentMode.Get(t.Data)
			if err != nil {
				return nil, errors.Wrap(err, "get agent mode")
			}
			if !ok {
				modeName = ""
			}
<<<<<<< HEAD
			if modeName == "" && svc != nil && sessionID != "" {
				if m, err := svc.GetCurrentMode(ctx, sessionID); err == nil && m != "" {
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
			if modeName == "" && svc != nil && t.RunID != "" {
				if m, err := svc.GetCurrentMode(ctx, t.RunID); err == nil && m != "" {
=======
			if modeName == "" && svc != nil && runID != "" {
				if m, err := svc.GetCurrentMode(ctx, runID); err == nil && m != "" {
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
					modeName = m
				}
			}
			if modeName == "" {
				modeName = cfg.DefaultMode
				if err := turns.KeyAgentMode.Set(&t.Data, modeName); err != nil {
					return nil, errors.Wrap(err, "set default agent mode")
				}
			}

			mode, err := svc.GetMode(ctx, modeName)
			if err != nil {
				log.Warn().Str("requested_mode", modeName).Msg("agentmode: unknown mode; continuing without restrictions")
			} else {
				// Remove previously inserted AgentMode-related blocks
				{
					valSet := map[string]struct{}{
						"agentmode_system_prompt":       {},
						"agentmode_switch_instructions": {},
						"agentmode_user_prompt":         {},
					}
					kept := make([]turns.Block, 0, len(t.Blocks))
					for _, b := range t.Blocks {
						tag, ok, err := turns.KeyBlockMetaAgentModeTag.Get(b.Metadata)
						if err != nil {
							return nil, errors.Wrap(err, "get agentmode tag block metadata")
						}
						if ok {
							if _, match := valSet[tag]; match {
								continue
							}
						}
						kept = append(kept, b)
					}
					t.Blocks = kept
				}

				// Build a single user block with mode prompt and (optionally) switch instructions
				var bldr strings.Builder
				if strings.TrimSpace(mode.Prompt) != "" {
					bldr.WriteString("<currentMode>")
					bldr.WriteString(strings.TrimSpace(mode.Prompt))
					bldr.WriteString("</currentMode>")
				}
				if bldr.Len() > 0 {
					bldr.WriteString("\n\n")
				}
				bldr.WriteString(BuildYamlModeSwitchInstructions(mode.Name, listModeNames(svc)))
				if bldr.Len() > 0 {
					text := bldr.String()
					prev := text
					if len(prev) > 120 {
						prev = prev[:120] + "…"
					}
					usr := turns.NewUserTextBlock(text)
					if err := turns.KeyBlockMetaAgentModeTag.Set(&usr.Metadata, "agentmode_user_prompt"); err != nil {
						return nil, errors.Wrap(err, "set agentmode_tag block metadata")
					}
					if err := turns.KeyBlockMetaAgentMode.Set(&usr.Metadata, mode.Name); err != nil {
						return nil, errors.Wrap(err, "set agentmode block metadata")
					}
					// Insert as second-to-last (before last assistant or tool block if present)
					before := len(t.Blocks)
					if before > 0 {
						before = before - 1
					}
					// Use append slicing to control placement
					if before < 0 {
						before = 0
					}
					if before >= len(t.Blocks) {
						turns.AppendBlock(t, usr)
					} else {
						t.Blocks = append(t.Blocks[:before], append([]turns.Block{usr}, t.Blocks[before:]...)...)
					}
<<<<<<< HEAD
					log.Debug().Str("session_id", sessionID).Str("turn_id", t.ID).Int("insert_pos", before).Str("preview", prev).Msg("agentmode: inserted user prompt block")
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
					log.Debug().Str("run_id", t.RunID).Str("turn_id", t.ID).Int("insert_pos", before).Str("preview", prev).Msg("agentmode: inserted user prompt block")
=======
					log.Debug().Str("run_id", runID).Str("turn_id", t.ID).Int("insert_pos", before).Str("preview", prev).Msg("agentmode: inserted user prompt block")
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
					// Log insertion
					events.PublishEventToContext(ctx, events.NewLogEvent(
<<<<<<< HEAD
						events.EventMetadata{ID: uuid.New(), SessionID: sessionID, TurnID: t.ID}, "info",
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
						events.EventMetadata{ID: uuid.New(), RunID: t.RunID, TurnID: t.ID}, "info",
=======
						events.EventMetadata{
							ID:          uuid.New(),
							SessionID:   runID,
							InferenceID: gcompat.TurnInferenceID(t),
							TurnID:      t.ID,
						},
						"info",
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
						"agentmode: user prompt inserted",
						map[string]any{"mode": mode.Name},
					))
				}
				// Pass allowed tools hint to downstream tool middleware
				if len(mode.AllowedTools) > 0 {
					if err := turns.KeyAgentModeAllowedTools.Set(&t.Data, append([]string(nil), mode.AllowedTools...)); err != nil {
						return nil, errors.Wrap(err, "set agentmode allowed tools")
					}
				}
			}

			// Run next
			baselineIDs := rootmw.SnapshotBlockIDs(t)
			res, err := next(ctx, t)
			if err != nil {
				return res, err
			}
			resSessionID := sessionIDFromTurn(res)
			if resSessionID == "" {
				resSessionID = sessionID
			}

			// Parse assistant response to detect YAML mode switch only in newly added blocks (by ID)
			addedBlocks := rootmw.NewBlocksNotIn(res, baselineIDs)
			newMode, analysis := DetectYamlModeSwitchInBlocks(addedBlocks)
			log.Debug().Str("new_mode", newMode).Str("analysis", analysis).Msg("agentmode: detected mode switch via YAML")
			resRunID := gcompat.TurnSessionID(res)
			resInferenceID := gcompat.TurnInferenceID(res)
			// Emit analysis event even when not switching (allocate a message_id)
			if strings.TrimSpace(analysis) != "" && newMode == "" {
<<<<<<< HEAD
				publishAgentModeSwitchEvent(ctx, events.EventMetadata{ID: uuid.New(), SessionID: resSessionID, TurnID: res.ID}, modeName, modeName, analysis)
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
				publishAgentModeSwitchEvent(ctx, events.EventMetadata{ID: uuid.New(), RunID: res.RunID, TurnID: res.ID}, modeName, modeName, analysis)
=======
				publishAgentModeSwitchEvent(ctx, events.EventMetadata{
					ID:          uuid.New(),
					SessionID:   resRunID,
					InferenceID: resInferenceID,
					TurnID:      res.ID,
				}, modeName, modeName, analysis)
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
			}
			if newMode != "" && newMode != modeName {
				log.Debug().Str("from", modeName).Str("to", newMode).Msg("agentmode: detected mode switch via YAML")
				// Apply to turn for next call
				if err := turns.KeyAgentMode.Set(&res.Data, newMode); err != nil {
					return nil, errors.Wrap(err, "set agent mode")
				}
				// Record change
				if svc != nil {
<<<<<<< HEAD
					_ = svc.RecordModeChange(ctx, ModeChange{SessionID: resSessionID, TurnID: res.ID, FromMode: modeName, ToMode: newMode, Analysis: analysis, At: time.Now()})
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
					_ = svc.RecordModeChange(ctx, ModeChange{RunID: res.RunID, TurnID: res.ID, FromMode: modeName, ToMode: newMode, Analysis: analysis, At: time.Now()})
=======
					_ = svc.RecordModeChange(ctx, ModeChange{
						RunID:    resRunID,
						TurnID:   res.ID,
						FromMode: modeName,
						ToMode:   newMode,
						Analysis: analysis,
						At:       time.Now(),
					})
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
				}
				// Announce: append system message and emit custom agent-mode event with analysis
				turns.AppendBlock(res, turns.NewSystemTextBlock(fmt.Sprintf("[agent-mode] switched to %s", newMode)))
<<<<<<< HEAD
				publishAgentModeSwitchEvent(ctx, events.EventMetadata{ID: uuid.New(), SessionID: resSessionID, TurnID: res.ID}, modeName, newMode, analysis)
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
				publishAgentModeSwitchEvent(ctx, events.EventMetadata{ID: uuid.New(), RunID: res.RunID, TurnID: res.ID}, modeName, newMode, analysis)
=======
				publishAgentModeSwitchEvent(ctx, events.EventMetadata{
					ID:          uuid.New(),
					SessionID:   resRunID,
					InferenceID: resInferenceID,
					TurnID:      res.ID,
				}, modeName, newMode, analysis)
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
			}
<<<<<<< HEAD
			log.Debug().Str("session_id", resSessionID).Str("turn_id", res.ID).Msg("agentmode: middleware end")
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
			log.Debug().Str("run_id", res.RunID).Str("turn_id", res.ID).Msg("agentmode: middleware end")
=======
			log.Debug().Str("run_id", resRunID).Str("turn_id", res.ID).Msg("agentmode: middleware end")
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
			return res, nil
		}
	}
}

// BuildYamlModeSwitchInstructions returns instructions for the model to propose a mode switch using YAML.
func BuildYamlModeSwitchInstructions(current string, available []string) string {
	var b strings.Builder
	b.WriteString("<modeSwitchGuidelines>")
	b.WriteString("Analyze the current conversation and determine if a mode switch would be beneficial. ")
	b.WriteString("Consider the user's request, the context, and the available capabilities in different modes. ")
	b.WriteString("If a mode switch would improve your ability to help the user, propose it using the following YAML format. ")
	b.WriteString("If the current mode is appropriate, do not include the new_mode field.")
	b.WriteString("</modeSwitchGuidelines>\n\n")
	b.WriteString("```yaml\n")
	b.WriteString("mode_switch:\n")
	b.WriteString("  analysis: |\n")
	b.WriteString("    • What is the user trying to accomplish?\n")
	b.WriteString("    • What capabilities are needed?\n")
	b.WriteString("    • Is the current mode optimal for this task?\n")
	b.WriteString("    • If switching, what specific benefits would the new mode provide?\n")
	b.WriteString("  new_mode: MODE_NAME  # Only include this if you recommend switching modes\n")
	b.WriteString("```\n\n")
	b.WriteString("Current mode: ")
	b.WriteString(current)
	if len(available) > 0 {
		b.WriteString("\nAvailable modes: ")
		b.WriteString(strings.Join(available, ", "))
	}
	b.WriteString("\n\nRemember: Only propose a mode switch if it would genuinely improve your ability to assist the user. ")
	b.WriteString("Staying in the current mode is often the right choice.")
	return b.String()
}

// DetectYamlModeSwitch scans assistant LLM text blocks for a YAML code fence containing mode_switch.
func DetectYamlModeSwitch(t *turns.Turn) (string, string) {
	if t == nil {
		return "", ""
	}
	for _, b := range t.Blocks {
		if b.Kind != turns.BlockKindLLMText {
			continue
		}
		txt, _ := b.Payload[turns.PayloadKeyText].(string)
		if txt == "" {
			continue
		}
		blocks, err := parse.ExtractYAMLBlocks(txt)
		if err != nil {
			continue
		}
		for _, body := range blocks {
			body = strings.TrimSpace(body)
			var data struct {
				ModeSwitch struct {
					Analysis string `yaml:"analysis"`
					NewMode  string `yaml:"new_mode,omitempty"`
				} `yaml:"mode_switch"`
			}
			if err := yaml.Unmarshal([]byte(body), &data); err != nil {
				continue
			}
			analysis := strings.TrimSpace(data.ModeSwitch.Analysis)
			if analysis == "" {
				continue
			}
			nm := strings.TrimSpace(data.ModeSwitch.NewMode)
			return nm, strings.TrimSpace(data.ModeSwitch.Analysis)
		}
	}
	return "", ""
}

// DetectYamlModeSwitchInBlocks scans the provided blocks from the back and
// returns the first detected (nearest to the end) YAML mode switch.
func DetectYamlModeSwitchInBlocks(blocks []turns.Block) (string, string) {
	for i := len(blocks) - 1; i >= 0; i-- {
		b := blocks[i]
		if b.Kind != turns.BlockKindLLMText {
			continue
		}
		txt, _ := b.Payload[turns.PayloadKeyText].(string)
		if strings.TrimSpace(txt) == "" {
			continue
		}
		yblocks, err := parse.ExtractYAMLBlocks(txt)
		if err != nil {
			continue
		}
		for _, body := range yblocks {
			body = strings.TrimSpace(body)
			var data struct {
				ModeSwitch struct {
					Analysis string `yaml:"analysis"`
					NewMode  string `yaml:"new_mode,omitempty"`
				} `yaml:"mode_switch"`
			}
			if err := yaml.Unmarshal([]byte(body), &data); err != nil {
				continue
			}
			analysis := strings.TrimSpace(data.ModeSwitch.Analysis)
			if analysis == "" {
				continue
			}
			nm := strings.TrimSpace(data.ModeSwitch.NewMode)
			return nm, analysis
		}
	}
	return "", ""
}

// listModeNames extracts available mode names from the provided Service, if it is a known implementation.
func listModeNames(svc Service) []string {
	if svc == nil {
		return nil
	}
	// Support StaticService and SQLiteService which both embed a modes map keyed by lower-case name
	switch s := svc.(type) {
	case *StaticService:
		names := make([]string, 0, len(s.modes))
		for _, m := range s.modes {
			if m != nil && m.Name != "" {
				names = append(names, m.Name)
			}
		}
		sort.Strings(names)
		return names
	case *SQLiteService:
		names := make([]string, 0, len(s.modes))
		for _, m := range s.modes {
			if m != nil && m.Name != "" {
				names = append(names, m.Name)
			}
		}
		sort.Strings(names)
		return names
	default:
		return nil
	}
}
