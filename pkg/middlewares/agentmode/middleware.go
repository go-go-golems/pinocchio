package agentmode

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/events"
	rootmw "github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
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
	DefaultMode  string
	ParseOptions ParseOptions
}

func DefaultConfig() Config {
	return Config{
		DefaultMode:  "default",
		ParseOptions: DefaultParseOptions(),
	}
}

func (c Config) withDefaults() Config {
	ret := c
	if strings.TrimSpace(ret.DefaultMode) == "" {
		ret.DefaultMode = DefaultConfig().DefaultMode
	}
	ret.ParseOptions = ret.ParseOptions.withDefaults()
	return ret
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
	cfg = cfg.withDefaults()
	return func(next rootmw.HandlerFunc) rootmw.HandlerFunc {
		return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
			if t == nil {
				return next(ctx, t)
			}

			sessionID := sessionIDFromTurn(t)
			log.Debug().Str("session_id", sessionID).Str("turn_id", t.ID).Msg("agentmode: middleware start")

			// Determine current mode: from Turn.Data or Store fallback
			modeName, ok, err := turns.KeyAgentMode.Get(t.Data)
			if err != nil {
				return nil, errors.Wrap(err, "get agent mode")
			}
			if !ok {
				modeName = ""
			}
			if modeName == "" && svc != nil && sessionID != "" {
				if m, err := svc.GetCurrentMode(ctx, sessionID); err == nil && m != "" {
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
				bldr.WriteString(BuildModeSwitchInstructions(mode.Name, listModeNames(svc)))
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
					log.Debug().Str("session_id", sessionID).Str("turn_id", t.ID).Int("insert_pos", before).Str("preview", prev).Msg("agentmode: inserted user prompt block")
					// Log insertion
					events.PublishEventToContext(ctx, events.NewLogEvent(
						events.EventMetadata{ID: uuid.New(), SessionID: sessionID, TurnID: t.ID}, "info",
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

			// Parse assistant response to detect a structured mode-switch payload only in newly added blocks (by ID)
			addedBlocks := rootmw.NewBlocksNotIn(res, baselineIDs)
			newMode, analysis := "", ""
			if parsed, ok := DetectModeSwitchInBlocks(addedBlocks, cfg.ParseOptions); ok {
				newMode = parsed.NewMode
				analysis = parsed.Analysis
			}
			log.Debug().Str("new_mode", newMode).Str("analysis", analysis).Msg("agentmode: detected mode switch via structured payload")
			// Emit analysis event even when not switching (allocate a message_id)
			if strings.TrimSpace(analysis) != "" && newMode == "" {
				publishAgentModeSwitchEvent(ctx, events.EventMetadata{ID: uuid.New(), SessionID: resSessionID, TurnID: res.ID}, modeName, modeName, analysis)
			}
			if newMode != "" && newMode != modeName {
				log.Debug().Str("from", modeName).Str("to", newMode).Msg("agentmode: detected mode switch via structured payload")
				// Apply to turn for next call
				if err := turns.KeyAgentMode.Set(&res.Data, newMode); err != nil {
					return nil, errors.Wrap(err, "set agent mode")
				}
				// Record change
				if svc != nil {
					_ = svc.RecordModeChange(ctx, ModeChange{SessionID: resSessionID, TurnID: res.ID, FromMode: modeName, ToMode: newMode, Analysis: analysis, At: time.Now()})
				}
				// Announce: append system message and emit custom agent-mode event with analysis
				turns.AppendBlock(res, turns.NewSystemTextBlock(fmt.Sprintf("[agent-mode] switched to %s", newMode)))
				publishAgentModeSwitchEvent(ctx, events.EventMetadata{ID: uuid.New(), SessionID: resSessionID, TurnID: res.ID}, modeName, newMode, analysis)
			}
			log.Debug().Str("session_id", resSessionID).Str("turn_id", res.ID).Msg("agentmode: middleware end")
			return res, nil
		}
	}
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
