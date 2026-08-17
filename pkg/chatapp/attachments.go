package chatapp

import (
	"strings"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
)

// AttachmentKindImage is the only attachment kind understood by the inference
// path today. Other kinds are carried through the protocol untouched but are
// not converted into turn image entries.
const AttachmentKindImage = "image"

// AttachmentMetadataTurnURL is an optional metadata key that overrides the URL
// placed in the geppetto turn image map. Applications use it when the browser
// facing URL (Attachment.URL) is not something the inference runtime should
// fetch directly, e.g. an internal reference the app resolves in a middleware.
const AttachmentMetadataTurnURL = "turn_url"

// Attachment describes a user-provided attachment by reference. Bytes never
// travel through chatapp: the hosting application stores them and exposes URLs.
type Attachment struct {
	ID        string
	Kind      string
	MediaType string
	URL       string
	Filename  string
	Detail    string
	SizeBytes uint64
	Width     uint32
	Height    uint32
	Metadata  map[string]string
}

// IsImage reports whether the attachment should be sent to the model as an image.
func (a Attachment) IsImage() bool {
	kind := strings.TrimSpace(a.Kind)
	if kind == "" {
		return strings.HasPrefix(strings.TrimSpace(a.MediaType), "image/")
	}
	return kind == AttachmentKindImage
}

// TurnURL returns the URL to place into the geppetto turn image map.
func (a Attachment) TurnURL() string {
	if a.Metadata != nil {
		if v := strings.TrimSpace(a.Metadata[AttachmentMetadataTurnURL]); v != "" {
			return v
		}
	}
	return strings.TrimSpace(a.URL)
}

// AttachmentsToTurnImages converts image attachments into the geppetto image map
// shape consumed by turns.NewUserMultimodalBlock / imageparts.NormalizeImageMap:
// {"attachment_id", "media_type", "url", "detail"?}. Extra keys are ignored by
// the normalizer. Non-image attachments and attachments without a URL are skipped.
func AttachmentsToTurnImages(atts []Attachment) []map[string]any {
	if len(atts) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(atts))
	for _, a := range atts {
		if !a.IsImage() {
			continue
		}
		url := a.TurnURL()
		if url == "" {
			continue
		}
		img := map[string]any{"url": url}
		if id := strings.TrimSpace(a.ID); id != "" {
			img["attachment_id"] = id
		}
		if mt := strings.TrimSpace(a.MediaType); mt != "" {
			img["media_type"] = mt
		}
		if d := strings.TrimSpace(a.Detail); d != "" {
			img["detail"] = d
		}
		out = append(out, img)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// AttachmentsToProto converts attachments to their protobuf representation.
func AttachmentsToProto(atts []Attachment) []*chatappv1.ChatAttachment {
	if len(atts) == 0 {
		return nil
	}
	out := make([]*chatappv1.ChatAttachment, 0, len(atts))
	for _, a := range atts {
		pb := &chatappv1.ChatAttachment{
			AttachmentId: a.ID,
			Kind:         a.Kind,
			MediaType:    a.MediaType,
			Url:          a.URL,
			SizeBytes:    a.SizeBytes,
			Width:        a.Width,
			Height:       a.Height,
			Filename:     a.Filename,
			Detail:       a.Detail,
		}
		if len(a.Metadata) > 0 {
			pb.Metadata = make(map[string]string, len(a.Metadata))
			for k, v := range a.Metadata {
				pb.Metadata[k] = v
			}
		}
		out = append(out, pb)
	}
	return out
}

// AttachmentsFromProto converts protobuf attachments back into Attachment values.
func AttachmentsFromProto(pbs []*chatappv1.ChatAttachment) []Attachment {
	if len(pbs) == 0 {
		return nil
	}
	out := make([]Attachment, 0, len(pbs))
	for _, pb := range pbs {
		if pb == nil {
			continue
		}
		a := Attachment{
			ID:        pb.GetAttachmentId(),
			Kind:      pb.GetKind(),
			MediaType: pb.GetMediaType(),
			URL:       pb.GetUrl(),
			Filename:  pb.GetFilename(),
			Detail:    pb.GetDetail(),
			SizeBytes: pb.GetSizeBytes(),
			Width:     pb.GetWidth(),
			Height:    pb.GetHeight(),
		}
		if md := pb.GetMetadata(); len(md) > 0 {
			a.Metadata = make(map[string]string, len(md))
			for k, v := range md {
				a.Metadata[k] = v
			}
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
