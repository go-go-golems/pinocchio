package cmds

import (
<<<<<<< HEAD
	"mime"
	"net/http"
	"os"
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
=======
	"net/url"
	"os"
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
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
<<<<<<< HEAD
		m, err := imagePathToPayload(p)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load image %q", p)
		}

		if len(m) > 0 {
			images = append(images, m)
		}
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
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
=======
		m := map[string]any{}
		mediaType := conversationMediaTypeFromExtension(filepath.Ext(p))
		if mediaType == "" {
			return nil, errors.Errorf("unsupported image extension for %q", p)
		}
		m["media_type"] = mediaType

		if isRemoteImageURL(p) {
			m["url"] = p
		} else {
			content, err := os.ReadFile(p)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to read image %q", p)
			}
			if len(content) > 0 {
				m["content"] = content
			} else {
				return nil, errors.Errorf("image %q is empty", p)
			}
		}

		images = append(images, m)
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
	}

	return images, nil
}

<<<<<<< HEAD
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
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
// conversationMediaTypeFromExtension mirrors conversation's internal media-type mapping (best-effort).
func conversationMediaTypeFromExtension(ext string) string {
=======
func isRemoteImageURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// conversationMediaTypeFromExtension mirrors conversation's internal media-type mapping (best-effort).
func conversationMediaTypeFromExtension(ext string) string {
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
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
