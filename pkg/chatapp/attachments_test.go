package chatapp

import (
	"context"
	"testing"
	"time"

	"github.com/go-go-golems/geppetto/pkg/turns"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

func TestAttachmentsToTurnImages(t *testing.T) {
	atts := []Attachment{
		{ID: "att-1", Kind: "image", MediaType: "image/png", URL: "/api/chat/sessions/s/attachments/att-1", Detail: "high"},
		{ID: "att-2", MediaType: "image/jpeg", URL: "https://x/b.jpg", Metadata: map[string]string{AttachmentMetadataTurnURL: "app://s/att-2"}},
		{ID: "att-3", Kind: "file", MediaType: "application/pdf", URL: "https://x/c.pdf"},
		{ID: "att-4", Kind: "image", MediaType: "image/png"}, // no URL → skipped
	}
	imgs := AttachmentsToTurnImages(atts)
	require.Len(t, imgs, 2)
	require.Equal(t, "att-1", imgs[0]["attachment_id"])
	require.Equal(t, "image/png", imgs[0]["media_type"])
	require.Equal(t, "/api/chat/sessions/s/attachments/att-1", imgs[0]["url"])
	require.Equal(t, "high", imgs[0]["detail"])
	require.Equal(t, "app://s/att-2", imgs[1]["url"], "turn_url metadata overrides URL")
	require.Nil(t, AttachmentsToTurnImages(nil))
}

func TestAttachmentsProtoRoundTrip(t *testing.T) {
	in := []Attachment{{ID: "a", Kind: "image", MediaType: "image/png", URL: "u", Filename: "f.png", Detail: "auto", SizeBytes: 12, Width: 3, Height: 4, Metadata: map[string]string{"k": "v"}}}
	pb := AttachmentsToProto(in)
	require.Len(t, pb, 1)
	require.Equal(t, "a", pb[0].GetAttachmentId())
	require.Equal(t, uint32(3), pb[0].GetWidth())
	out := AttachmentsFromProto(pb)
	require.Equal(t, in, out)
	require.Nil(t, AttachmentsToProto(nil))
	require.Nil(t, AttachmentsFromProto(nil))
}

func TestServiceSubmitPromptRequestAllowsAttachmentsWithoutPrompt(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	svc, err := NewService(hub, engine)
	require.NoError(t, err)

	err = svc.SubmitPromptRequest(context.Background(), sessionstream.SessionId("svc-att-0"), PromptRequest{})
	require.Error(t, err, "neither prompt nor attachments")

	att := Attachment{ID: "att-1", Kind: "image", MediaType: "image/png", URL: "https://example.com/a.png", Width: 10, Height: 20}
	require.NoError(t, svc.SubmitPromptRequest(context.Background(), sessionstream.SessionId("svc-att-1"), PromptRequest{Attachments: []Attachment{att}}))
	require.NoError(t, svc.WaitIdle(context.Background(), sessionstream.SessionId("svc-att-1")))

	snap, err := svc.Snapshot(context.Background(), sessionstream.SessionId("svc-att-1"))
	require.NoError(t, err)
	var user *chatappv1.ChatMessageEntity
	for _, entity := range snap.Entities {
		if m, ok := entity.Payload.(*chatappv1.ChatMessageEntity); ok && m.GetRole() == "user" {
			user = m
		}
	}
	require.NotNil(t, user, "user message entity must exist for image-only submission")
	require.Equal(t, "", user.GetContent())
	require.Len(t, user.GetAttachments(), 1)
	require.Equal(t, "att-1", user.GetAttachments()[0].GetAttachmentId())
	require.Equal(t, "https://example.com/a.png", user.GetAttachments()[0].GetUrl())
	require.Equal(t, uint32(20), user.GetAttachments()[0].GetHeight())
}

func TestRuntimeInferenceAppendsMultimodalUserBlockForAttachments(t *testing.T) {
	ctx := context.Background()
	recorder := &recordingHistoryEngine{}
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	engine.setPendingRequest("request-att", PromptRequest{
		Prompt:      "what coin is this?",
		Attachments: []Attachment{{ID: "att-9", Kind: "image", MediaType: "image/jpeg", URL: "https://example.com/coin.jpg", Metadata: map[string]string{AttachmentMetadataTurnURL: "coinvault-attachment://sess/att-9"}}},
		Runtime:     &infruntime.ComposedRuntime{Engine: recorder},
	})
	require.NoError(t, hub.Submit(ctx, sessionstream.SessionId("chat-att"), CommandStartInference, &chatappv1.StartInferenceCommand{RequestId: "request-att"}))
	require.NoError(t, engine.WaitIdle(ctx, sessionstream.SessionId("chat-att")))

	seen := recorder.seen
	require.NotNil(t, seen)
	require.Len(t, seen.Blocks, 1)
	require.Equal(t, turns.RoleUser, seen.Blocks[0].Role)
	require.Equal(t, "what coin is this?", seen.Blocks[0].Payload[turns.PayloadKeyText])
	imgs := turns.BlockImages(seen.Blocks[0])
	require.Len(t, imgs, 1)
	require.Equal(t, "coinvault-attachment://sess/att-9", imgs[0]["url"])
	require.Equal(t, "image/jpeg", imgs[0]["media_type"])
	require.Equal(t, "att-9", imgs[0]["attachment_id"])
}

func TestHandleStartInferenceFallsBackToWireAttachments(t *testing.T) {
	// No pending request registered: attachments must be reconstructed from the command payload.
	ctx := context.Background()
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	cmd := &chatappv1.StartInferenceCommand{RequestId: "unknown", Attachments: []*chatappv1.ChatAttachment{{AttachmentId: "w-1", Kind: "image", MediaType: "image/png", Url: "https://example.com/w.png"}}}
	require.NoError(t, hub.Submit(ctx, sessionstream.SessionId("chat-wire"), CommandStartInference, cmd))
	require.NoError(t, engine.WaitIdle(ctx, sessionstream.SessionId("chat-wire")))
	svc, err := NewService(hub, engine)
	require.NoError(t, err)
	snap, err := svc.Snapshot(ctx, sessionstream.SessionId("chat-wire"))
	require.NoError(t, err)
	var user *chatappv1.ChatMessageEntity
	for _, entity := range snap.Entities {
		if m, ok := entity.Payload.(*chatappv1.ChatMessageEntity); ok && m.GetRole() == "user" {
			user = m
		}
	}
	require.NotNil(t, user)
	require.Equal(t, "", user.GetContent(), "no demo fallback prompt when attachments are present")
	require.Len(t, user.GetAttachments(), 1)
	require.Equal(t, "w-1", user.GetAttachments()[0].GetAttachmentId())
}
