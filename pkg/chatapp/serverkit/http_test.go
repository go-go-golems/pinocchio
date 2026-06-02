package serverkit

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseSessionPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantID  string
		wantAct string
		wantOK  bool
	}{
		{name: "snapshot", path: "/api/chat/sessions/sess-1", wantID: "sess-1", wantOK: true},
		{name: "action", path: "/api/chat/sessions/sess-1/messages", wantID: "sess-1", wantAct: "messages", wantOK: true},
		{name: "missing", path: "/api/chat/sessions/", wantOK: false},
		{name: "too deep", path: "/api/chat/sessions/sess-1/messages/extra", wantOK: false},
		{name: "wrong prefix", path: "/api/other/sess-1", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotAct, gotOK := ParseSessionPath(tt.path)
			if gotOK != tt.wantOK || gotID != tt.wantID || gotAct != tt.wantAct {
				t.Fatalf("ParseSessionPath(%q) = (%q, %q, %v), want (%q, %q, %v)", tt.path, gotID, gotAct, gotOK, tt.wantID, tt.wantAct, tt.wantOK)
			}
		})
	}
}

func TestDecodeJSONAllowsEmptyBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(""))
	var out SubmitMessageRequest
	if err := DecodeJSON(req, &out); err != nil {
		t.Fatalf("DecodeJSON empty body: %v", err)
	}
}

func TestDecodeJSONRejectsMalformedBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader("{"))
	var out SubmitMessageRequest
	if err := DecodeJSON(req, &out); err == nil {
		t.Fatalf("expected malformed body error")
	}
}
