package presets

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "coder.md", "You write code.")
	write(t, dir, "coder.yaml", `
name: coder
description: Writes code
model: anthropic/claude-sonnet-4-5
thinking: medium
maxTokens: 8000
systemPrompt: coder.md
maxRounds: 40
toolTimeout: 120s
resultCut: 9000
tools: [read_file, write_file]
budget:
  tokensPerSession: 2000000
room: main
`)
	write(t, dir, "plain.yml", "model: openai/gpt-5-mini\n")
	write(t, dir, "notes.txt", "ignored")

	got, err := Load(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"coder", "plain"}, Names(got))

	c := got["coder"]
	require.Equal(t, "Writes code", c.Description)
	require.Equal(t, "anthropic/claude-sonnet-4-5", c.Model)
	require.Equal(t, "You write code.", c.SystemText())
	require.Equal(t, 40, c.MaxRounds)
	require.Equal(t, 120*time.Second, c.ToolTimeoutDuration())
	require.Equal(t, 9000, c.ResultCut)
	require.Equal(t, []string{"read_file", "write_file"}, c.Tools)
	require.Equal(t, 2000000, c.Budget.TokensPerSession)
	require.Equal(t, "main", c.Extra["room"], "unknown keys are kept for the application")

	p := got["plain"]
	require.Equal(t, "", p.SystemText())
	require.Zero(t, p.MaxRounds, "no defaults: zero means not set")
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	require.ErrorContains(t, err, "no presets")

	write(t, dir, "a.yaml", "name: same\n")
	write(t, dir, "b.yaml", "name: same\n")
	_, err = Load(dir)
	require.ErrorContains(t, err, `preset "same" is defined in`)

	other := t.TempDir()
	write(t, other, "x.yaml", "systemPrompt: missing.md\n")
	_, err = Load(other)
	require.ErrorContains(t, err, "system prompt")

	bad := t.TempDir()
	write(t, bad, "x.yaml", "toolTimeout: soon\n")
	_, err = Load(bad)
	require.ErrorContains(t, err, `duration "soon"`)
}
