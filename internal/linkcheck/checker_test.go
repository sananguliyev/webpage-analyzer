package linkcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sananguliyev/webpage-analyzer/internal/model"
)

func TestChecker_Check(t *testing.T) {
	// Setup test servers
	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer okServer.Close()

	notFoundServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer notFoundServer.Close()

	serverErrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer serverErrorServer.Close()

	// HEAD 405 → GET fallback
	headNotAllowedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer headNotAllowedServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/target", http.StatusMovedPermanently)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectServer.Close()

	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	checker := &Checker{
		Client: &http.Client{
			Timeout: 500 * time.Millisecond,
		},
		MaxWorkers: 5,
		MaxLinks:   100,
	}

	tests := []struct {
		name           string
		url            string
		wantAccessible bool
		wantStatus     int
		wantError      bool
	}{
		{"HEAD 200", okServer.URL, true, 200, false},
		{"HEAD 405 → GET fallback", headNotAllowedServer.URL, true, 200, false},
		{"404", notFoundServer.URL, false, 404, false},
		{"500", serverErrorServer.URL, false, 500, false},
		{"Redirect (3xx accessible)", redirectServer.URL + "/redirect", true, 200, false},
		{"Timeout", slowServer.URL, false, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := []model.LinkResult{{URL: tt.url}}
			results := checker.Check(context.Background(), links)

			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}

			r := results[0]
			if r.IsAccessible != tt.wantAccessible {
				t.Errorf("IsAccessible = %v, want %v", r.IsAccessible, tt.wantAccessible)
			}
			if tt.wantStatus > 0 && r.StatusCode != tt.wantStatus {
				t.Errorf("StatusCode = %d, want %d", r.StatusCode, tt.wantStatus)
			}
			if tt.wantError && r.Error == "" {
				t.Error("expected error, got none")
			}
			if !tt.wantError && r.Error != "" {
				t.Errorf("unexpected error: %s", r.Error)
			}
		})
	}
}

func TestChecker_MaxLinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := &Checker{
		Client:     &http.Client{Timeout: time.Second},
		MaxWorkers: 5,
		MaxLinks:   3,
	}

	links := make([]model.LinkResult, 10)
	for i := range links {
		links[i] = model.LinkResult{URL: server.URL}
	}

	results := checker.Check(context.Background(), links)
	if len(results) != 3 {
		t.Errorf("expected 3 results (capped), got %d", len(results))
	}
}

func TestChecker_ContextCancellation(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	checker := &Checker{
		Client:     &http.Client{Timeout: 10 * time.Second},
		MaxWorkers: 1,
		MaxLinks:   100,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	links := []model.LinkResult{{URL: slowServer.URL}}
	results := checker.Check(ctx, links)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].IsAccessible {
		t.Error("expected inaccessible due to context cancellation")
	}
}

func TestChecker_NetworkError(t *testing.T) {
	checker := &Checker{
		Client:     &http.Client{Timeout: time.Second},
		MaxWorkers: 5,
		MaxLinks:   100,
	}

	links := []model.LinkResult{{URL: "http://192.0.2.1:1"}} // RFC 5737 TEST-NET, unreachable
	results := checker.Check(context.Background(), links)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].IsAccessible {
		t.Error("expected inaccessible for unreachable host")
	}
	if results[0].Error == "" {
		t.Error("expected error message")
	}
}
