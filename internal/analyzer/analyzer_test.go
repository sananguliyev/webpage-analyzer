package analyzer

import (
	"net/url"
	"testing"
)

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func TestDetectHTMLVersion(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{"HTML5", "<!DOCTYPE html><html>", "HTML5"},
		{"HTML5 uppercase", "<!DOCTYPE HTML><html>", "HTML5"},
		{"HTML 4.01 Strict", `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN">`, "HTML 4.01 Strict"},
		{"HTML 4.01 Transitional", `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01 Transitional//EN">`, "HTML 4.01 Transitional"},
		{"HTML 4.01 Frameset", `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01 Frameset//EN">`, "HTML 4.01 Frameset"},
		{"XHTML 1.0 Strict", `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN">`, "XHTML 1.0 Strict"},
		{"XHTML 1.0 Transitional", `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN">`, "XHTML 1.0 Transitional"},
		{"XHTML 1.0 Frameset", `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Frameset//EN">`, "XHTML 1.0 Frameset"},
		{"XHTML 1.1", `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN">`, "XHTML 1.1"},
		{"No DOCTYPE", "<html><body>Hello</body></html>", "Unknown"},
		{"Empty", "", "Unknown"},
		{"Case variation", "<!doctype html><html>", "HTML5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectHTMLVersion([]byte(tt.html))
			if got != tt.want {
				t.Errorf("DetectHTMLVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAnalyze_Title(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{"Normal title", "<html><head><title>Hello World</title></head></html>", "Hello World"},
		{"Empty title", "<html><head><title></title></head></html>", ""},
		{"Missing title", "<html><head></head></html>", ""},
		{"Whitespace title", "<html><head><title>  Hello  </title></head></html>", "Hello"},
	}

	base := mustParseURL("https://example.com")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Analyze([]byte(tt.html), base)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Title != tt.want {
				t.Errorf("Title = %q, want %q", result.Title, tt.want)
			}
		})
	}
}

func TestAnalyze_Headings(t *testing.T) {
	tests := []struct {
		name string
		html string
		want map[string]int
	}{
		{
			"Mixed headings",
			"<html><body><h1>A</h1><h1>B</h1><h2>C</h2><h3>D</h3><h6>E</h6></body></html>",
			map[string]int{"h1": 2, "h2": 1, "h3": 1, "h6": 1},
		},
		{
			"No headings",
			"<html><body><p>text</p></body></html>",
			map[string]int{},
		},
	}

	base := mustParseURL("https://example.com")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Analyze([]byte(tt.html), base)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for tag, count := range tt.want {
				if result.Headings[tag] != count {
					t.Errorf("Headings[%s] = %d, want %d", tag, result.Headings[tag], count)
				}
			}
			if len(result.Headings) != len(tt.want) {
				t.Errorf("Headings has %d entries, want %d", len(result.Headings), len(tt.want))
			}
		})
	}
}

func TestAnalyze_Links(t *testing.T) {
	tests := []struct {
		name         string
		html         string
		baseURL      string
		wantInternal int
		wantExternal int
		wantTotal    int
	}{
		{
			"Internal and external",
			`<html><body>
				<a href="/about">About</a>
				<a href="https://example.com/contact">Contact</a>
				<a href="https://other.com">Other</a>
			</body></html>`,
			"https://example.com",
			2, 1, 3,
		},
		{
			"Relative link",
			`<html><body><a href="page.html">Page</a></body></html>`,
			"https://example.com/dir/",
			1, 0, 1,
		},
		{
			"Protocol-relative",
			`<html><body><a href="//cdn.example.com/file">CDN</a></body></html>`,
			"https://example.com",
			0, 1, 1,
		},
		{
			"Skip mailto, tel, javascript",
			`<html><body>
				<a href="mailto:a@b.com">Email</a>
				<a href="tel:+1234">Phone</a>
				<a href="javascript:void(0)">JS</a>
				<a href="/real">Real</a>
			</body></html>`,
			"https://example.com",
			1, 0, 1,
		},
		{
			"Skip fragment-only",
			`<html><body><a href="#section">Section</a><a href="/page">Page</a></body></html>`,
			"https://example.com",
			1, 0, 1,
		},
		{
			"Empty href skipped",
			`<html><body><a href="">Empty</a><a href="/page">Page</a></body></html>`,
			"https://example.com",
			1, 0, 1,
		},
		{
			"Duplicate dedup",
			`<html><body>
				<a href="/page">Page 1</a>
				<a href="/page">Page 2</a>
			</body></html>`,
			"https://example.com",
			1, 0, 1,
		},
		{
			"Base href override",
			`<html><head><base href="https://other.com/"></head><body>
				<a href="/page">Page</a>
			</body></html>`,
			"https://example.com",
			0, 1, 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := mustParseURL(tt.baseURL)
			result, err := Analyze([]byte(tt.html), base)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.InternalLinks != tt.wantInternal {
				t.Errorf("InternalLinks = %d, want %d", result.InternalLinks, tt.wantInternal)
			}
			if result.ExternalLinks != tt.wantExternal {
				t.Errorf("ExternalLinks = %d, want %d", result.ExternalLinks, tt.wantExternal)
			}
			if result.TotalLinks != tt.wantTotal {
				t.Errorf("TotalLinks = %d, want %d", result.TotalLinks, tt.wantTotal)
			}
		})
	}
}

func TestAnalyze_LoginForm(t *testing.T) {
	tests := []struct {
		name string
		html string
		want bool
	}{
		{
			"With password input",
			`<html><body><form><input type="password"></form></body></html>`,
			true,
		},
		{
			"Without password input",
			`<html><body><form><input type="text"></form></body></html>`,
			false,
		},
		{
			"Case-insensitive type",
			`<html><body><form><input type="Password"></form></body></html>`,
			true,
		},
		{
			"Password outside form",
			`<html><body><input type="password"></body></html>`,
			true,
		},
	}

	base := mustParseURL("https://example.com")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Analyze([]byte(tt.html), base)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.HasLoginForm != tt.want {
				t.Errorf("HasLoginForm = %v, want %v", result.HasLoginForm, tt.want)
			}
		})
	}
}
