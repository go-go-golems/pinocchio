package cmds

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
		m, err := imagePathToPayload(p)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load image %q", p)
		}

		if len(m) > 0 {
			images = append(images, m)
		}
	}

	return images, nil
}

func imagePathToPayload(p string) (map[string]any, error) {
	if p == "" {
		return nil, nil
	}
	mediaType := mediaTypeFromExtension(filepath.Ext(p))
	if isRemoteImage(p) {
		if mediaType == "" || !strings.HasPrefix(mediaType, "image/") {
			return nil, errors.New("unsupported image type")
		}
		m := map[string]any{
			"url": p,
		}
		if mediaType != "" {
			m["media_type"] = mediaType
		}
		return m, nil
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	if mediaType == "" {
		mediaType = http.DetectContentType(raw)
	}
	if mediaType == "" || !strings.HasPrefix(mediaType, "image/") {
		return nil, errors.New("unsupported image type")
	}
	m := map[string]any{
		"content": raw,
	}
	if mediaType != "" {
		m["media_type"] = mediaType
	}
	return m, nil
}

func isRemoteImage(p string) bool {
	p = strings.TrimSpace(strings.ToLower(p))
	return strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://")
}

func mediaTypeFromExtension(ext string) string {
	if ext == "" {
		return ""
	}
	if mt := mime.TypeByExtension(ext); mt != "" {
		return mt
	}
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
