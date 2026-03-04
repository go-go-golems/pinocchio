package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	renderers "github.com/go-go-golems/bobatea/pkg/timeline/renderers"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"github.com/go-go-golems/pinocchio/pkg/ui"
	"github.com/go-go-golems/pinocchio/pkg/ui/profileswitch"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/structpb"
)

type openProfilePickerMsg struct{}

type appModel struct {
	inner tea.Model

	backend       *profileswitch.Backend
	manager       *profileswitch.Manager
	sink          events.EventSink
	persistSwitch func(from, to, runtimeKey, runtimeFingerprint string) error

	active *huh.Form

	// picker state
	selectedSlug string
	options      []huh.Option[string]

	convID string
}

func newAppModel(inner tea.Model, backend *profileswitch.Backend, manager *profileswitch.Manager, sink events.EventSink, persistSwitch func(from, to, runtimeKey, runtimeFingerprint string) error, convID string) appModel {
	return appModel{
		inner:         inner,
		backend:       backend,
		manager:       manager,
		sink:          sink,
		persistSwitch: persistSwitch,
		convID:        strings.TrimSpace(convID),
	}
}

func (m appModel) Init() tea.Cmd { return m.inner.Init() }

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case openProfilePickerMsg:
		items, err := m.manager.ListProfiles(context.Background())
		if err != nil {
			return m, localPlainEntityCmd("profile_error", map[string]any{"error": err.Error()})
		}
		if len(items) == 0 {
			return m, localPlainEntityCmd("profile_error", map[string]any{"error": "no profiles loaded"})
		}

		opts := make([]huh.Option[string], 0, len(items))
		for _, it := range items {
			title := it.ProfileSlug.String()
			if strings.TrimSpace(it.DisplayName) != "" {
				title = fmt.Sprintf("%s — %s", it.ProfileSlug.String(), it.DisplayName)
			}
			opts = append(opts, huh.NewOption(title, it.ProfileSlug.String()))
		}
		m.options = opts
		m.selectedSlug = m.backend.Current().ProfileSlug.String()

		m.active = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Switch profile").
					Options(opts...).
					Value(&m.selectedSlug),
			),
		)
		// blur input while modal is active
		innerModel, cmd := m.inner.Update(chat.BlurInputMsg{})
		m.inner = innerModel
		return m, cmd

	case tea.KeyMsg:
		// While modal is active, route all keys to the form first.
		if m.active != nil {
			fm, cmd := m.active.Update(v)
			if f, ok := fm.(*huh.Form); ok {
				m.active = f
			}
			if m.active != nil && m.active.State == huh.StateCompleted {
				target := strings.TrimSpace(m.selectedSlug)
				from := m.backend.Current().ProfileSlug.String()
				res, err := m.backend.SwitchProfile(context.Background(), target)
				// unblur input either way
				innerModel, unblurCmd := m.inner.Update(chat.UnblurInputMsg{})
				m.inner = innerModel
				m.active = nil

				if err != nil {
					return m, tea.Batch(cmd, unblurCmd, localPlainEntityCmd("profile_error", map[string]any{"error": err.Error()}))
				}

				publishCmd := func() tea.Msg {
					if err := publishProfileSwitchedInfo(m.sink, m.convID, from, res.ProfileSlug.String(), res.RuntimeKey.String(), res.RuntimeFingerprint); err != nil {
						log.Warn().Err(err).Msg("failed to publish profile-switched info event")
					}
					if m.persistSwitch != nil {
						if err := m.persistSwitch(from, res.ProfileSlug.String(), res.RuntimeKey.String(), res.RuntimeFingerprint); err != nil {
							log.Warn().Err(err).Msg("failed to persist profile switch marker")
						}
					}
					return nil
				}
				return m, tea.Batch(
					cmd,
					unblurCmd,
					publishCmd,
					localPlainEntityCmd("profile_switched", map[string]any{
						"from":        from,
						"to":          res.ProfileSlug.String(),
						"runtime_key": res.RuntimeKey.String(),
					}),
				)
			}
			return m, cmd
		}
	}

	// Default: forward to inner model
	innerModel, cmd := m.inner.Update(msg)
	m.inner = innerModel
	return m, cmd
}

func (m appModel) View() string {
	if m.active != nil {
		return m.active.View()
	}
	return m.inner.View()
}

func localPlainEntityCmd(kind string, props map[string]any) tea.Cmd {
	id := uuid.NewString()
	created := func() tea.Msg {
		return timeline.UIEntityCreated{
			ID:        timeline.EntityID{LocalID: id, Kind: "plain"},
			Renderer:  timeline.RendererDescriptor{Kind: "plain"},
			Props:     mergeStringAnyMap(map[string]any{"kind": strings.TrimSpace(kind)}, props),
			StartedAt: time.Now(),
		}
	}
	completed := func() tea.Msg {
		return timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: id, Kind: "plain"}}
	}
	return tea.Batch(created, completed)
}

func mergeStringAnyMap(base map[string]any, extra map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
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

type lockedTimelineStore struct {
	chatstore.TimelineStore
	mu *sync.Mutex
}

func (s *lockedTimelineStore) Upsert(ctx context.Context, convID string, version uint64, entity *timelinepb.TimelineEntityV2) error {
	if s == nil || s.TimelineStore == nil {
		return nil
	}
	if s.mu != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
	}
	return s.TimelineStore.Upsert(ctx, convID, version, entity)
}

func main() {
	var (
		profileRegistries string
		profileSlug       string
		convID            string

		timelineDB string
		turnsDB    string

		logLevel string
	)

	root := &cobra.Command{
		Use:   "switch-profiles-tui",
		Short: "Bubble Tea chat TUI with /profile switching via Geppetto profile registries",
		RunE: func(cmd *cobra.Command, args []string) error {
			zerolog.TimeFieldFormat = time.StampMilli
			if lvl := strings.TrimSpace(logLevel); lvl != "" {
				if parsed, err := zerolog.ParseLevel(lvl); err == nil {
					zerolog.SetGlobalLevel(parsed)
				}
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if strings.TrimSpace(profileRegistries) == "" {
				return errors.New("--profile-registries is required and must not be empty")
			}
			if convID == "" {
				convID = "tui-" + uuid.NewString()
			}

			base, err := settings.NewStepSettings()
			if err != nil {
				return err
			}

			mgr, err := profileswitch.NewManagerFromSources(ctx, profileRegistries, base)
			if err != nil {
				return err
			}
			defer func() { _ = mgr.Close() }()

			profiles, err := mgr.ListProfiles(ctx)
			if err != nil {
				return err
			}
			if len(profiles) == 0 {
				return errors.New("no profiles loaded from --profile-registries (refusing to start)")
			}

			timelineStore, turnStore, closeStores, err := openStores(timelineDB, turnsDB)
			if err != nil {
				return err
			}
			defer closeStores()

			// Shared timeline version counter across multiple persistence sources.
			var timelineVersion atomic.Uint64
			var timelineStoreMu sync.Mutex
			if timelineStore != nil {
				timelineStore = &lockedTimelineStore{TimelineStore: timelineStore, mu: &timelineStoreMu}
			}

			// Use a buffered in-memory pubsub for TUI runs.
			//
			// Watermill's gochannel defaults to an unbuffered output channel, which can
			// deadlock streaming inference if a subscriber is registered but not yet
			// actively consuming messages.
			goPubSub := gochannel.NewGoChannel(gochannel.Config{
				OutputChannelBuffer:            256,
				BlockPublishUntilSubscriberAck: false,
			}, watermill.NopLogger{})
			router, err := events.NewEventRouter(
				events.WithPublisher(goPubSub),
				events.WithSubscriber(goPubSub),
			)
			if err != nil {
				return err
			}
			defer func() { _ = router.Close() }()

			sink := middleware.NewWatermillSink(router.Publisher, "chat")

			persistSwitch := func(from, to, runtimeKey, runtimeFingerprint string) error {
				if timelineStore == nil || strings.TrimSpace(convID) == "" {
					return nil
				}
				seq := timelineVersion.Add(1)
				propsMap := map[string]any{
					"schemaVersion":       1,
					"from":                strings.TrimSpace(from),
					"to":                  strings.TrimSpace(to),
					"runtime_key":         strings.TrimSpace(runtimeKey),
					"runtime_fingerprint": strings.TrimSpace(runtimeFingerprint),
					"profile.slug":        strings.TrimSpace(to),
				}
				props, err := structpb.NewStruct(propsMap)
				if err != nil {
					return err
				}
				entity := &timelinepb.TimelineEntityV2{
					Id:    uuid.NewString(),
					Kind:  "profile_switch",
					Props: props,
				}
				persistCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				return timelineStore.Upsert(persistCtx, convID, seq, entity)
			}

			var persister *turnStorePersister
			if turnStore != nil {
				persister = newTurnStorePersister(turnStore, convID)
			}
			backend, err := profileswitch.NewBackend(mgr, sink, persister, nil)
			if err != nil {
				return err
			}

			// Initialize profile (default if empty)
			res, err := backend.InitDefaultProfile(ctx, profileSlug)
			if err != nil {
				return err
			}

			// Chat model with submit interception
			var app *appModel
			header := func() string {
				cur := backend.Current()
				if cur.ProfileSlug.IsZero() {
					return ""
				}
				return fmt.Sprintf("profile=%s  runtime=%s", cur.ProfileSlug.String(), cur.RuntimeKey.String())
			}
			interceptor := func(input string) (bool, tea.Cmd) {
				parts := strings.Fields(strings.TrimSpace(input))
				if len(parts) == 0 || parts[0] != "/profile" {
					return false, nil
				}

				// /profile -> open picker
				if len(parts) == 1 {
					return true, func() tea.Msg { return openProfilePickerMsg{} }
				}

				// /profile help
				if len(parts) >= 2 && parts[1] == "help" {
					return true, localPlainEntityCmd("profile_help", map[string]any{
						"usage": "/profile [<slug>|help]",
					})
				}

				// /profile <slug>
				target := strings.TrimSpace(parts[1])
				from := backend.Current().ProfileSlug.String()
				next, err := backend.SwitchProfile(ctx, target)
				if err != nil {
					return true, localPlainEntityCmd("profile_error", map[string]any{"error": err.Error()})
				}
				publishCmd := func() tea.Msg {
					if err := publishProfileSwitchedInfo(sink, convID, from, next.ProfileSlug.String(), next.RuntimeKey.String(), next.RuntimeFingerprint); err != nil {
						log.Warn().Err(err).Msg("failed to publish profile-switched info event")
					}
					if err := persistSwitch(from, next.ProfileSlug.String(), next.RuntimeKey.String(), next.RuntimeFingerprint); err != nil {
						log.Warn().Err(err).Msg("failed to persist profile switch marker")
					}
					return nil
				}
				return true, tea.Batch(
					publishCmd,
					localPlainEntityCmd("profile_switched", map[string]any{
						"from":        from,
						"to":          next.ProfileSlug.String(),
						"runtime_key": next.RuntimeKey.String(),
					}),
				)
			}

			model := chat.InitialModel(backend,
				chat.WithTitle("switch-profiles-tui"),
				chat.WithTimelineRegister(func(r *timeline.Registry) {
					r.RegisterModelFactory(renderers.NewLLMTextFactory())
					r.RegisterModelFactory(renderers.PlainFactory{})
				}),
				chat.WithSubmitInterceptor(interceptor),
				chat.WithHeaderView(header),
			)

			// Wrap in a modal overlay host
			appModel := newAppModel(model, backend, mgr, sink, persistSwitch, convID)
			app = &appModel

			program := tea.NewProgram(appModel, tea.WithAltScreen(), tea.WithMouseCellMotion())

			// Forward events to UI timeline entities
			router.AddHandler("ui-forward", "chat", ui.StepChatForwardFunc(program))
			if timelineStore != nil {
				router.AddHandler("timeline-persist", "chat", ui.StepTimelinePersistFuncWithVersion(timelineStore, convID, &timelineVersion))
			}
			// Debug hook for local runs: log EventInfo frames when log-level is debug/trace.
			router.AddHandler("debug-info-log", "chat", func(msg *message.Message) error {
				msg.Ack()
				ev, err := events.NewEventFromJson(msg.Payload)
				if err != nil {
					return nil
				}
				if info, ok := ev.(*events.EventInfo); ok {
					log.Debug().
						Str("conversation_id", convID).
						Str("message", strings.TrimSpace(info.Message)).
						Interface("data", info.Data).
						Msg("EventInfo received")
				}
				return nil
			})

			log.Info().Str("conv_id", convID).Str("profile", res.ProfileSlug.String()).Str("runtime_key", res.RuntimeKey.String()).Msg("Starting TUI")

			eg, groupCtx := errgroup.WithContext(ctx)
			eg.Go(func() error { return router.Run(groupCtx) })
			eg.Go(func() error {
				_, err := program.Run()
				cancel()
				return err
			})
			if err := eg.Wait(); err != nil {
				return err
			}

			_ = app
			return nil
		},
	}

	root.Flags().StringVar(&profileRegistries, "profile-registries", "", "Comma-separated profile registry sources (yaml/sqlite/sqlite-dsn). REQUIRED.")
	root.Flags().StringVar(&profileSlug, "profile", "", "Initial profile slug (default: registry default profile)")
	root.Flags().StringVar(&convID, "conv-id", "", "Conversation ID for persistence (default: generated)")
	root.Flags().StringVar(&timelineDB, "timeline-db", "/tmp/switch-profiles-tui.timeline.db", "SQLite DB file for timeline projection persistence")
	root.Flags().StringVar(&turnsDB, "turns-db", "/tmp/switch-profiles-tui.turns.db", "SQLite DB file for turn snapshot persistence")
	root.Flags().StringVar(&logLevel, "log-level", "info", "Log level (trace|debug|info|warn|error)")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func openStores(timelineDB, turnsDB string) (chatstore.TimelineStore, chatstore.TurnStore, func(), error) {
	var timelineStore chatstore.TimelineStore
	var turnStore chatstore.TurnStore
	cleanup := func() {
		if turnStore != nil {
			_ = turnStore.Close()
		}
		if timelineStore != nil {
			_ = timelineStore.Close()
		}
	}

	tdb := strings.TrimSpace(timelineDB)
	if tdb != "" {
		dsn, err := chatstore.SQLiteTimelineDSNForFile(tdb)
		if err != nil {
			return nil, nil, cleanup, err
		}
		s, err := chatstore.NewSQLiteTimelineStore(dsn)
		if err != nil {
			return nil, nil, cleanup, err
		}
		timelineStore = s
	}

	rdb := strings.TrimSpace(turnsDB)
	if rdb != "" {
		dsn, err := chatstore.SQLiteTurnDSNForFile(rdb)
		if err != nil {
			return nil, nil, cleanup, err
		}
		s, err := chatstore.NewSQLiteTurnStore(dsn)
		if err != nil {
			return nil, nil, cleanup, err
		}
		turnStore = s
	}

	return timelineStore, turnStore, cleanup, nil
}
