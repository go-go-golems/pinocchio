// Package sessions keeps metadata about chatapp sessions: who owns a session,
// free-form labels, a title and the token usage summed over its runs.
//
// The conversation itself lives in chatapp's turn store and the timeline in the
// sessionstream hydration store; this package only holds what an application
// needs to list, filter, authorize and account for sessions. chatapp does not
// require it: an application that has a single session, or keeps its own list,
// can ignore it.
package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Info is the metadata of one session.
type Info struct {
	ID string `json:"id"`
	// Owner is the principal that created the session ("" when anonymous).
	// httpapi sets it from the authenticated request.
	Owner string `json:"owner,omitempty"`
	// Labels are free-form metadata, e.g. {"room": "main"}; List filters on them.
	Labels map[string]string `json:"labels,omitempty"`
	// Title is set by the application (see TitleFrom).
	Title   string    `json:"title,omitempty"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	// Usage is summed over every provider call of the session (RecordUsage).
	Usage Usage `json:"usage"`
	// Data is the application's own JSON for this session, opaque to chatapp
	// (e.g. instructions an application adds to its system prompt).
	Data json.RawMessage `json:"data,omitempty"`
}

// Usage counts tokens. Input includes cached input for providers that report
// cached tokens inside the input count (OpenAI-style); CacheRead and
// CacheCreation are reported separately by providers such as Claude.
type Usage struct {
	Input         int `json:"input"`
	Output        int `json:"output"`
	Cached        int `json:"cached,omitempty"`
	CacheCreation int `json:"cacheCreation,omitempty"`
	CacheRead     int `json:"cacheRead,omitempty"`
	// Calls is the number of provider calls counted.
	Calls int `json:"calls"`
}

// Add returns u plus v.
func (u Usage) Add(v Usage) Usage {
	return Usage{
		Input:         u.Input + v.Input,
		Output:        u.Output + v.Output,
		Cached:        u.Cached + v.Cached,
		CacheCreation: u.CacheCreation + v.CacheCreation,
		CacheRead:     u.CacheRead + v.CacheRead,
		Calls:         u.Calls + v.Calls,
	}
}

// Tokens is Input + Output, the quantity token budgets usually count.
func (u Usage) Tokens() int { return u.Input + u.Output }

// Filter selects sessions in List. Owner always matches exactly: the zero
// Filter lists the sessions without an owner, not everyone's. Listing every
// owner's sessions takes AllOwners, so it cannot happen by an empty id.
type Filter struct {
	Owner     string
	AllOwners bool              // ignore Owner: every owner's sessions (administration)
	Labels    map[string]string // every key must match exactly
	Limit     int               // 0: no limit
}

// Matches reports whether info passes the filter.
func (f Filter) Matches(info Info) bool {
	if !f.AllOwners && info.Owner != f.Owner {
		return false
	}
	for k, v := range f.Labels {
		if info.Labels[k] != v {
			return false
		}
	}
	return true
}

var (
	// ErrNotFound is returned for an unknown session id.
	ErrNotFound = errors.New("sessions: session not found")
	// ErrExists is returned by Create for an id that is already taken,
	// including the id of a deleted session.
	ErrExists = errors.New("sessions: session already exists")
)

// Store keeps session metadata. Implementations must be safe for concurrent use.
type Store interface {
	// Create adds a session. Created and Updated default to now when zero.
	Create(ctx context.Context, info Info) error
	// Get returns a session or ErrNotFound.
	Get(ctx context.Context, id string) (Info, error)
	// List returns matching sessions, newest first.
	List(ctx context.Context, f Filter) ([]Info, error)
	// Update applies fn to the stored session atomically and sets Updated.
	// fn must not change ID; an error from fn aborts the update.
	Update(ctx context.Context, id string, fn func(*Info) error) error
	// Delete removes a session or returns ErrNotFound. The id stays taken:
	// a tombstone keeps Create from giving it to anyone again, since a
	// session's conversation and timeline (other stores) outlive it.
	Delete(ctx context.Context, id string) error
}

// TitleFrom returns the first non-empty line of text, cut to maxRunes runes
// (with "…"). A helper for applications that title sessions by their first
// message; chatapp never sets titles itself.
func TitleFrom(text string, maxRunes int) string {
	line := ""
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			line = l
			break
		}
	}
	if maxRunes > 0 && utf8.RuneCountInString(line) > maxRunes {
		r := []rune(line)
		line = string(r[:maxRunes]) + "…"
	}
	return line
}

func cloneInfo(in Info) Info {
	out := in
	if in.Data != nil {
		out.Data = append(json.RawMessage(nil), in.Data...)
	}
	if in.Labels != nil {
		out.Labels = make(map[string]string, len(in.Labels))
		for k, v := range in.Labels {
			out.Labels[k] = v
		}
	}
	return out
}
