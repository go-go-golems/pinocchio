package agentd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/modelsource"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
)

// Store keeps sessions as JSON files in a directory: metadata, the latest
// turn (the whole conversation, as geppetto turn YAML) and the recent events.
type Store struct {
	Dir string
	// KeepEvents bounds the events saved per session.
	KeepEvents int
}

type storedSession struct {
	ID         string            `json:"id"`
	Labels     map[string]string `json:"labels,omitempty"`
	Agent      string            `json:"agent"`
	Model      modelsource.Model `json:"model"`
	Thinking   string            `json:"thinking,omitempty"`
	System     string            `json:"system"`
	Title      string            `json:"title,omitempty"`
	Created    time.Time         `json:"created"`
	Updated    time.Time         `json:"updated"`
	Usage      Usage             `json:"usage"`
	ContextIDs []string          `json:"contextIds,omitempty"`
	TurnYAML   string            `json:"turn,omitempty"`
	Events     []Event           `json:"events,omitempty"`
}

// NewStore creates the directory if needed.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{Dir: dir, KeepEvents: 2000}, nil
}

func (st *Store) path(id string) string { return filepath.Join(st.Dir, id+".json") }

// Save writes a session atomically.
func (st *Store) Save(s *Session) error {
	s.mu.Lock()
	rec := storedSession{ID: s.ID, Labels: s.Labels, Agent: s.Agent.Name, Model: s.Model, Thinking: s.thinking, System: s.system, Title: s.title,
		Created: s.Created, Updated: s.updated, Usage: s.usage, ContextIDs: append([]string(nil), s.contextIDs...)}
	s.mu.Unlock()
	if t := s.gs.Latest(); t != nil {
		b, err := serde.ToYAML(t, serde.Options{OmitData: true})
		if err != nil {
			return fmt.Errorf("serialize turn: %w", err)
		}
		rec.TurnYAML = string(b)
	}
	rec.Events = s.log.Since(0)
	if n := st.KeepEvents; n > 0 && len(rec.Events) > n {
		rec.Events = rec.Events[len(rec.Events)-n:]
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp := st.path(s.ID) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, st.path(s.ID))
}

// Delete removes a saved session.
func (st *Store) Delete(id string) error {
	err := os.Remove(st.path(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// loadAll returns every saved session record.
func (st *Store) loadAll() ([]storedSession, error) {
	ents, err := os.ReadDir(st.Dir)
	if err != nil {
		return nil, err
	}
	var out []storedSession
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(st.Dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var rec storedSession
		if err := json.Unmarshal(b, &rec); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, rec)
	}
	return out, nil
}
