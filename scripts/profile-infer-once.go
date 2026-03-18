package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/ui/profileswitch"
)

func main() {
	var (
		profileRegistries string
		profileSlug       string
		prompt            string
		timeout           time.Duration
	)

	flag.StringVar(&profileRegistries, "profile-registries", "/tmp/profile-registry.yaml", "Comma-separated profile registry sources")
	flag.StringVar(&profileSlug, "profile", "", "Profile slug to run (required)")
	flag.StringVar(&prompt, "prompt", "Say just one word: OK.", "Prompt to run")
	flag.DurationVar(&timeout, "timeout", 90*time.Second, "Inference timeout")
	flag.Parse()

	profileRegistries = strings.TrimSpace(profileRegistries)
	profileSlug = strings.TrimSpace(profileSlug)
	prompt = strings.TrimSpace(prompt)
	if profileRegistries == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --profile-registries must not be empty")
		os.Exit(2)
	}
	if profileSlug == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --profile is required")
		os.Exit(2)
	}
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --prompt must not be empty")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	base, err := settings.NewInferenceSettings()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	mgr, err := profileswitch.NewManagerFromSources(ctx, profileRegistries, base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	defer func() { _ = mgr.Close() }()

	resolved, err := mgr.Switch(ctx, profileSlug)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "profile=%s\n", resolved.ProfileSlug.String())
	fmt.Fprintf(os.Stderr, "runtime_key=%s\n", resolved.RuntimeKey.String())
	fmt.Fprintf(os.Stderr, "runtime_fingerprint=%s\n", resolved.RuntimeFingerprint)

	eng, err := factory.NewEngineFromSettings(resolved.InferenceSettings)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	router, err := events.NewEventRouter()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	defer func() { _ = router.Close() }()
	router.AddHandler("ack-only", "chat", func(msg *message.Message) error {
		msg.Ack()
		return nil
	})
	go func() { _ = router.Run(ctx) }()

	sink := middleware.NewWatermillSink(router.Publisher, "chat")

	mws := []middleware.Middleware{}
	if strings.TrimSpace(resolved.SystemPrompt) != "" {
		mws = append(mws, middleware.NewSystemPromptMiddleware(resolved.SystemPrompt))
	}

	builder := &enginebuilder.Builder{
		Base:        eng,
		Middlewares: mws,
		EventSinks:  []events.EventSink{sink},
	}

	sess := session.NewSession()
	sess.Builder = builder

	t, err := sess.AppendNewTurnFromUserPrompt(prompt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	_ = turns.KeyTurnMetaRuntime.Set(&t.Metadata, map[string]any{
		"runtime_key":         resolved.RuntimeKey.String(),
		"profile.slug":        resolved.ProfileSlug.String(),
		"profile.registry":    resolved.RegistrySlug.String(),
		"profile.version":     resolved.ProfileVersion,
		"runtime_fingerprint": resolved.RuntimeFingerprint,
	})

	handle, err := sess.StartInference(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	updated, err := handle.Wait()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	if updated == nil {
		fmt.Fprintln(os.Stderr, "ERROR: updated turn is nil")
		os.Exit(1)
	}

	text := ""
	for i := len(updated.Blocks) - 1; i >= 0; i-- {
		b := updated.Blocks[i]
		if b.Kind != turns.BlockKindLLMText || strings.TrimSpace(b.Role) != turns.RoleAssistant {
			continue
		}
		raw, ok := b.Payload[turns.PayloadKeyText]
		if !ok {
			continue
		}
		if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
			text = strings.TrimSpace(s)
			break
		}
	}
	if text == "" {
		fmt.Fprintln(os.Stderr, "ERROR: no assistant text block found in updated turn")
		blob, _ := json.MarshalIndent(updated, "", "  ")
		fmt.Fprintln(os.Stderr, string(blob))
		os.Exit(1)
	}

	fmt.Println(text)
}
