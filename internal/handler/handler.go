package handler

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sananguliyev/webpage-analyzer/internal/analyzer"
	"github.com/sananguliyev/webpage-analyzer/internal/linkcheck"
	"github.com/sananguliyev/webpage-analyzer/internal/model"
)

type PageFetcher interface {
	Fetch(ctx context.Context, rawURL string) ([]byte, *url.URL, error)
}

type Handler struct {
	Template         *template.Template
	Fetcher          PageFetcher
	LinkChecker      *linkcheck.Checker
	FetchTimeout     time.Duration
	LinkCheckTimeout time.Duration
	Logger           *slog.Logger
}

type templateData struct {
	URL      string
	Analysis *model.PageAnalysis
	Error    string
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	if err := h.Template.Execute(w, templateData{}); err != nil {
		h.Logger.Error("template execution failed", "error", err)
	}
}

func (h *Handler) Analyze(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.FormValue("url"))
	if rawURL == "" {
		h.renderError(w, "", "Please enter a URL", http.StatusBadRequest)
		return
	}

	// Add scheme if missing
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		h.renderError(w, rawURL, "Invalid URL", http.StatusBadRequest)
		return
	}

	fetchCtx, fetchCancel := context.WithTimeout(r.Context(), h.FetchTimeout)
	defer fetchCancel()

	body, finalURL, err := h.Fetcher.Fetch(fetchCtx, rawURL)
	if err != nil {
		h.Logger.Error("fetch failed", "url", rawURL, "error", err)
		h.renderError(w, rawURL, fmt.Sprintf("Failed to fetch URL: %s", err), http.StatusUnprocessableEntity)
		return
	}

	analysis, err := analyzer.Analyze(body, finalURL)
	if err != nil {
		h.Logger.Error("analysis failed", "url", rawURL, "error", err)
		h.renderError(w, rawURL, fmt.Sprintf("Failed to analyze page: %s", err), http.StatusUnprocessableEntity)
		return
	}

	if len(analysis.Links) > 0 {
		linkCtx, linkCancel := context.WithTimeout(r.Context(), h.LinkCheckTimeout)
		defer linkCancel()

		checked := h.LinkChecker.Check(linkCtx, analysis.Links)
		analysis.Links = checked
		analysis.CheckedLinks = len(checked)
		analysis.TotalLinks = len(checked)

		analysis.InternalLinks = 0
		analysis.ExternalLinks = 0
		for _, l := range checked {
			if l.IsInternal {
				analysis.InternalLinks++
			} else {
				analysis.ExternalLinks++
			}
		}
	}

	if err := h.Template.Execute(w, templateData{
		URL:      rawURL,
		Analysis: analysis,
	}); err != nil {
		h.Logger.Error("template execution failed", "error", err)
	}
}

func (h *Handler) renderError(w http.ResponseWriter, inputURL, msg string, status int) {
	w.WriteHeader(status)
	if err := h.Template.Execute(w, templateData{
		URL:   inputURL,
		Error: msg,
	}); err != nil {
		h.Logger.Error("template execution failed", "error", err)
	}
}
