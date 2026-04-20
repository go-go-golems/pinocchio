package main

import (
	"bytes"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	rendererhtml "github.com/yuin/goldmark/renderer/html"
)

func (s *systemlabServer) handleChapterHTML(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	name := strings.TrimPrefix(req.URL.Path, "/api/chapters/")
	html, err := renderChapterHTML(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(html)
}

func renderChapterHTML(name string) ([]byte, error) {
	md, err := readChapterMarkdown(name)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	parser := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(rendererhtml.WithUnsafe()),
	)
	if err := parser.Convert(md, &out); err != nil {
		return nil, fmt.Errorf("render chapter %q: %w", name, err)
	}
	return out.Bytes(), nil
}

func readChapterMarkdown(name string) ([]byte, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("chapter name is empty")
	}
	clean := path.Clean(name)
	if clean == "." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "..") {
		return nil, fmt.Errorf("invalid chapter name %q", name)
	}
	if !strings.HasSuffix(clean, ".md") {
		clean += ".md"
	}
	body, err := appFS.ReadFile(path.Join("chapters", clean))
	if err != nil {
		return nil, fmt.Errorf("read chapter %q: %w", name, err)
	}
	return body, nil
}
