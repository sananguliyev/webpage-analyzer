# Web Page Analyzer

A Go web application that analyzes web pages for HTML version, title, headings, links (internal/external/inaccessible), and login form detection.

## Build & Run

Requires Go 1.25+ or Docker.

```bash
# Local
make run            # builds and starts server at http://localhost:8080

# Tests
make test           # runs all tests with verbose output

# Docker
make docker-run     # builds image and runs container at http://localhost:8080
```

Environment variables (all optional):

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server listen port |
| `FETCH_TIMEOUT` | `10s` | Timeout for fetching the target URL |
| `LINK_CHECK_TIMEOUT` | `30s` | Overall timeout for link accessibility checks |
| `MAX_CONCURRENT_CHECKS` | `10` | Max concurrent link check goroutines |

## Assumptions & Decisions

- **Login form detection:** A page has a login form if it contains an `<input type="password">` element (case-insensitive). This is a heuristic - password inputs outside `<form>` tags are also counted, since many modern pages use JS-based form submission.
- **Inaccessible links:** A link is considered inaccessible if it returns HTTP 4xx/5xx, times out (10s per link), or has a network error. HEAD is tried first; if the server returns 405, a GET fallback is used. Results may be inaccurate due to Cloudflare/CAPTCHA challenges, bot detection, geo-blocking, rate limiting, or firewall rules on the running environment.
- **Link cap:** Only the first 100 unique links are checked for accessibility to keep response times reasonable.
- **Internal vs external:** A link is internal if its hostname matches the analyzed page's hostname (case-insensitive). Protocol-relative and relative URLs are resolved against the page's base URL.
- **`<base href>` handling:** If the page contains a `<base href>` tag, it overrides the base for URL resolution, but internal/external classification still uses the original page hostname.
- **Body size limit:** Pages larger than 5MB are rejected to prevent memory issues.
- **Non-HTML content:** If the response Content-Type is not `text/html`, the URL is rejected with a clear error.
- **No browser engine:** The tool fetches raw HTML over HTTP. Single-page applications (React, Angular, Vue) that render content via JavaScript will not be fully analyzed.
- **HTML version detection:** Based on DOCTYPE string matching. Covers HTML5, HTML 4.01 (Strict/Transitional/Frameset), and XHTML 1.0/1.1. Pages without a DOCTYPE are reported as "Unknown".
- **Duplicate links:** Links are deduplicated by resolved URL (after stripping fragments) before counting and checking.
- **Skipped links:** `mailto:`, `tel:`, `javascript:`, and fragment-only (`#section`) hrefs are excluded from link counts.

## Possible Improvements

- **Browser-based rendering:** Use a headless browser (e.g. chromedp) to support SPA analysis and JavaScript-rendered content.
- **Async progress:** Use WebSocket or SSE to stream link check results as they complete, improving UX for pages with many links.
- **Caching:** Cache analysis results for recently analyzed URLs to avoid redundant fetches.
- **SSRF protection:** Validate resolved IPs against private/internal ranges (RFC-1918, loopback, link-local, cloud metadata) using a custom `net.Dialer` on the HTTP transport. Also reject non-http/https schemes and screen each redirect hop, not just the initial URL.
- **Rate limiting:** Add per-IP rate limiting for production deployments.
- **Robots.txt:** Respect robots.txt when checking link accessibility.
- **Retry logic:** Retry transient failures (429, 503) with exponential backoff.
- **JSON API:** Add a `/api/analyze` endpoint returning JSON for programmatic use.
- **Configurable link cap:** Allow users to set how many links to check via the UI.
- **`<base href>` validation:** Validate that `<base href>` hostnames match the original URL to prevent SSRF pivots via crafted pages.
- **WriteTimeout tuning:** Raise `WriteTimeout` to `FetchTimeout + LinkCheckTimeout + margin` to avoid truncating responses on slow analyses.
- **Dockerfile HEALTHCHECK:** Add a `HEALTHCHECK` instruction for orchestrator health detection.
