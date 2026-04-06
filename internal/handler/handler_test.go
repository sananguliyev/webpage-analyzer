package handler

import (
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sananguliyev/webpage-analyzer/internal/fetcher"
	"github.com/sananguliyev/webpage-analyzer/internal/linkcheck"
	"github.com/sananguliyev/webpage-analyzer/templates"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()

	tmpl, err := template.ParseFS(templates.FS, "index.html")
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	return &Handler{
		Template: tmpl,
		Fetcher: &fetcher.Fetcher{
			Client: &http.Client{
				Timeout: 5 * time.Second,
			},
		},
		LinkChecker: &linkcheck.Checker{
			Client:     &http.Client{Timeout: 2 * time.Second},
			MaxWorkers: 5,
			MaxLinks:   100,
		},
		FetchTimeout:     5 * time.Second,
		LinkCheckTimeout: 10 * time.Second,
		Logger:           slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
}

func TestIndex(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	h.Index(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Web Page Analyzer") {
		t.Error("expected page title in response")
	}
	if !strings.Contains(body, `name="url"`) {
		t.Error("expected URL input field")
	}
}

func TestAnalyze_EmptyURL(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader("url="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Analyze(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), "Please enter a URL") {
		t.Error("expected validation error message")
	}
}

func TestAnalyze_InvalidURL(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader("url=://not-a-url"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Analyze(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAnalyze_ValidURL(t *testing.T) {
	// Create a test HTML server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<h1>Hello</h1>
<h2>World</h2>
<a href="/about">About</a>
<a href="https://example.com">Example</a>
</body>
</html>`))
	}))
	defer ts.Close()

	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader("url="+url.QueryEscape(ts.URL)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Analyze(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Test Page") {
		t.Error("expected page title in results")
	}
	if !strings.Contains(body, "HTML5") {
		t.Error("expected HTML version in results")
	}
}

func TestAnalyze_NonHTMLContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"key": "value"}`))
	}))
	defer ts.Close()

	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader("url="+url.QueryEscape(ts.URL)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Analyze(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(w.Body.String(), "not an HTML page") {
		t.Error("expected non-HTML error message")
	}
}

func TestAnalyze_UnreachableURL(t *testing.T) {
	h := newTestHandler(t)
	h.FetchTimeout = 1 * time.Second
	h.Fetcher = &fetcher.Fetcher{
		Client: &http.Client{Timeout: 1 * time.Second},
	}

	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader("url="+url.QueryEscape("http://192.0.2.1:1")))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Analyze(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(w.Body.String(), "Failed to fetch URL") {
		t.Error("expected fetch error message")
	}
}

func TestAnalyze_URLWithoutScheme(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head><title>OK</title></head></html>`))
	}))
	defer ts.Close()

	// Strip scheme for test — but since httptest uses localhost:PORT, we need to use the full URL
	// This test validates the scheme-adding logic for input without "://"
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader("url=example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Analyze(w, req)

	// The request will fail because example.com won't be our test server,
	// but it should not fail with "Invalid URL" — it should attempt the fetch
	body := w.Body.String()
	if strings.Contains(body, "Invalid URL") {
		t.Error("should not get Invalid URL for input without scheme")
	}
}

func TestAnalyze_LargeBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// Write more than 5MB
		w.Write([]byte("<!DOCTYPE html><html><body>"))
		buf := make([]byte, 1024)
		for i := range buf {
			buf[i] = 'a'
		}
		for i := 0; i < 6000; i++ {
			w.Write(buf)
		}
		w.Write([]byte("</body></html>"))
	}))
	defer ts.Close()

	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader("url="+url.QueryEscape(ts.URL)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Analyze(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(w.Body.String(), "too large") {
		t.Error("expected size limit error")
	}
}

func TestAnalyze_WithLoginForm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><body>
			<form action="/login" method="POST">
				<input type="text" name="username">
				<input type="password" name="password">
				<button type="submit">Login</button>
			</form>
		</body></html>`))
	}))
	defer ts.Close()

	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader("url="+url.QueryEscape(ts.URL)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Analyze(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "Detected") {
		t.Error("expected login form detected")
	}
}

func TestAnalyze_WhitespaceURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head><title>OK</title></head></html>`))
	}))
	defer ts.Close()

	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader("url=+"+url.QueryEscape(ts.URL)+"++"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Analyze(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
