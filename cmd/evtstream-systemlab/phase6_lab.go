package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type phase6RunRequest struct {
	Action         string `json:"action"`
	BaseURL        string `json:"baseUrl"`
	Profile        string `json:"profile"`
	Prompt         string `json:"prompt"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

type phase6RunResponse struct {
	Action            string          `json:"action"`
	BaseURL           string          `json:"baseUrl"`
	Profile           string          `json:"profile"`
	Prompt            string          `json:"prompt"`
	SessionID         string          `json:"sessionId"`
	Trace             []traceEntry    `json:"trace"`
	AvailableProfiles []string        `json:"availableProfiles"`
	RouteStatuses     map[string]int  `json:"routeStatuses"`
	CreateResponse    map[string]any  `json:"createResponse"`
	SubmitResponse    map[string]any  `json:"submitResponse"`
	Snapshot          map[string]any  `json:"snapshot"`
	Checks            map[string]bool `json:"checks"`
	Error             string          `json:"error,omitempty"`
}

type phase6State struct {
	lastRun phase6RunResponse
}

type phase6ProfileRecord struct {
	Registry string `json:"registry"`
	Slug     string `json:"slug"`
}

func (e *labEnvironment) RunPhase6(ctx context.Context, in phase6RunRequest) (phase6RunResponse, error) {
	action := strings.TrimSpace(in.Action)
	if action == "" {
		action = "state"
	}
	baseURL := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:18112"
	}
	profile := strings.TrimSpace(in.Profile)
	if profile == "" {
		profile = "gpt-5-nano-low"
	}
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		prompt = "In one short sentence, explain ordinals."
	}
	timeoutSeconds := in.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 45
	}

	switch action {
	case "state":
		e.mu.Lock()
		defer e.mu.Unlock()
		resp := e.phase6.lastRun
		if resp.Action == "" {
			resp = phase6RunResponse{Action: action, BaseURL: baseURL, Profile: profile, Prompt: prompt, RouteStatuses: map[string]int{}, Checks: map[string]bool{}}
		}
		return resp, nil
	case "reset-phase6":
		e.mu.Lock()
		e.phase6.lastRun = phase6RunResponse{Action: action, BaseURL: baseURL, Profile: profile, Prompt: prompt, RouteStatuses: map[string]int{}, Checks: map[string]bool{}}
		e.mu.Unlock()
		return e.RunPhase6(ctx, phase6RunRequest{Action: "state", BaseURL: baseURL, Profile: profile, Prompt: prompt})
	case "run":
		resp, err := e.runPhase6Probe(ctx, baseURL, profile, prompt, timeoutSeconds)
		if err != nil {
			return phase6RunResponse{}, err
		}
		e.mu.Lock()
		e.phase6.lastRun = resp
		e.mu.Unlock()
		return resp, nil
	default:
		return phase6RunResponse{}, fmt.Errorf("unknown phase 6 action %q", action)
	}
}

func (e *labEnvironment) runPhase6Probe(ctx context.Context, baseURL, profile, prompt string, timeoutSeconds int) (phase6RunResponse, error) {
	trace := []traceEntry{}
	appendTrace := func(kind, message string, details map[string]any) {
		trace = append(trace, traceEntry{Step: len(trace) + 1, Kind: kind, Message: message, Details: cloneMap(details)})
	}
	client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	resp := phase6RunResponse{
		Action:        "run",
		BaseURL:       baseURL,
		Profile:       profile,
		Prompt:        prompt,
		RouteStatuses: map[string]int{},
		Checks:        map[string]bool{},
	}

	var profiles []phase6ProfileRecord
	status, err := phase6GetJSON(ctx, client, baseURL+"/api/chat/profiles", &profiles)
	if err != nil {
		return phase6RunResponse{}, err
	}
	resp.RouteStatuses["GET /api/chat/profiles"] = status
	appendTrace("http", "listed canonical profiles", map[string]any{"status": status, "profileCount": len(profiles)})
	for _, p := range profiles {
		resp.AvailableProfiles = append(resp.AvailableProfiles, p.Slug)
	}

	status, err = phase6StatusOnly(ctx, client, http.MethodPost, baseURL+"/chat", map[string]any{"prompt": prompt})
	if err != nil {
		return phase6RunResponse{}, err
	}
	resp.RouteStatuses["POST /chat"] = status
	appendTrace("http", "checked legacy chat route", map[string]any{"status": status})

	status, err = phase6StatusOnly(ctx, client, http.MethodGet, baseURL+"/api/timeline?conv_id=legacy-probe", nil)
	if err != nil {
		return phase6RunResponse{}, err
	}
	resp.RouteStatuses["GET /api/timeline"] = status
	appendTrace("http", "checked legacy timeline route", map[string]any{"status": status})

	var created map[string]any
	status, err = phase6PostJSON(ctx, client, baseURL+"/api/chat/sessions", map[string]any{"profile": profile}, &created)
	if err != nil {
		return phase6RunResponse{}, err
	}
	resp.RouteStatuses["POST /api/chat/sessions"] = status
	resp.CreateResponse = cloneMap(created)
	appendTrace("http", "created canonical session", map[string]any{"status": status, "response": created})

	sessionID := toString(created["sessionId"])
	if sessionID == "" {
		return phase6RunResponse{}, fmt.Errorf("canonical create session response missing sessionId")
	}
	resp.SessionID = sessionID

	var submitted map[string]any
	status, err = phase6PostJSON(ctx, client, baseURL+"/api/chat/sessions/"+sessionID+"/messages", map[string]any{"prompt": prompt, "profile": profile}, &submitted)
	if err != nil {
		return phase6RunResponse{}, err
	}
	resp.RouteStatuses["POST /api/chat/sessions/:sessionId/messages"] = status
	resp.SubmitResponse = cloneMap(submitted)
	appendTrace("http", "submitted canonical prompt", map[string]any{"status": status, "response": submitted})

	snapshotURL := baseURL + "/api/chat/sessions/" + sessionID
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	for {
		var snap map[string]any
		status, err = phase6GetJSON(ctx, client, snapshotURL, &snap)
		if err != nil {
			return phase6RunResponse{}, err
		}
		resp.RouteStatuses["GET /api/chat/sessions/:sessionId"] = status
		resp.Snapshot = cloneMap(snap)
		appendTrace("poll", "polled canonical snapshot", map[string]any{"status": status, "snapshotStatus": toString(snap["status"]), "ordinal": toString(snap["ordinal"]), "entityCount": len(phase6Entities(snap))})
		if phase6SnapshotTerminal(snap) || time.Now().After(deadline) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	assistant := phase6MessageByRole(resp.Snapshot, "assistant")
	user := phase6MessageByRole(resp.Snapshot, "user")
	assistantText := strings.TrimSpace(toString(assistant["content"]))
	if assistantText == "" {
		assistantText = strings.TrimSpace(toString(assistant["text"]))
	}
	resp.Trace = append([]traceEntry(nil), trace...)
	resp.Checks = map[string]bool{
		"profilesLoaded":           len(resp.AvailableProfiles) > 0,
		"targetProfilePresent":     containsString(resp.AvailableProfiles, profile),
		"createSessionOK":          resp.SessionID != "",
		"submitMessageOK":          resp.SubmitResponse != nil,
		"legacyRoutesRemoved":      resp.RouteStatuses["POST /chat"] == http.StatusNotFound && resp.RouteStatuses["GET /api/timeline"] == http.StatusNotFound,
		"snapshotHasUser":          len(user) > 0,
		"snapshotHasAssistant":     len(assistant) > 0,
		"assistantCompleted":       phase6AssistantCompleted(resp.Snapshot, assistant),
		"assistantGeneratedText":   assistantText != "",
		"assistantIsNotEchoEngine": assistantText != "" && assistantText != "Answer: "+prompt,
	}
	return resp, nil
}

func phase6StatusOnly(ctx context.Context, client *http.Client, method, url string, body any) (int, error) {
	var payload io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		payload = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, payload)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func phase6GetJSON(ctx context.Context, client *http.Client, url string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.StatusCode, err
	}
	return resp.StatusCode, nil
}

func phase6PostJSON(ctx context.Context, client *http.Client, url string, input any, out any) (int, error) {
	buf, err := json.Marshal(input)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.StatusCode, err
	}
	return resp.StatusCode, nil
}

func phase6SnapshotTerminal(snapshot map[string]any) bool {
	status := toString(snapshot["status"])
	return status == "finished" || status == "stopped"
}

func phase6Entities(snapshot map[string]any) []map[string]any {
	raw, _ := snapshot["entities"].([]any)
	entities := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		entry, _ := item.(map[string]any)
		if len(entry) > 0 {
			entities = append(entities, entry)
		}
	}
	return entities
}

func phase6MessageByRole(snapshot map[string]any, role string) map[string]any {
	for _, entity := range phase6Entities(snapshot) {
		payload, _ := entity["payload"].(map[string]any)
		if toString(payload["role"]) == role {
			return payload
		}
	}
	return nil
}

func phase6AssistantCompleted(snapshot map[string]any, assistant map[string]any) bool {
	if len(assistant) > 0 {
		status := toString(assistant["status"])
		if status == "finished" || status == "stopped" {
			return true
		}
	}
	status := toString(snapshot["status"])
	return status == "finished" || status == "stopped"
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
