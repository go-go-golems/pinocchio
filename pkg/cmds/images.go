package cmds

import (
	"path/filepath"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/pkg/errors"
)

// imagePathsToTurnImages converts CLI image paths to the turns.PayloadKeyImages payload format.
//
// Each image is represented as a map with:
// - "media_type": string
// - either "content": []byte (preferred for local files) or "url": string (for remote images)
func imagePathsToTurnImages(imagePaths []string) ([]map[string]any, error) {
	if len(imagePaths) == 0 {
		return nil, nil
	}

	images := make([]map[string]any, 0, len(imagePaths))
	for _, p := range imagePaths {
		img, err := conversation.NewImageContentFromFile(p)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load image %q", p)
		}

		mediaType := img.MediaType
		if mediaType == "" {
			// Best-effort for URL images where MediaType isn't populated.
			mediaType = conversationMediaTypeFromExtension(filepath.Ext(p))
		}

		m := map[string]any{}
		if mediaType != "" {
			m["media_type"] = mediaType
		}
		if len(img.ImageContent) > 0 {
			m["content"] = img.ImageContent
		} else if img.ImageURL != "" {
			m["url"] = img.ImageURL
		}

		images = append(images, m)
	}

	return images, nil
}

// conversationMediaTypeFromExtension mirrors conversation's internal media-type mapping (best-effort).
func conversationMediaTypeFromExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return ""
	}
}
