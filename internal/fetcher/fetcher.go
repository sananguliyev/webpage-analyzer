package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxBodySize = 5 * 1024 * 1024 // 5MB

type Fetcher struct {
	Client *http.Client
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL string) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "WebPageAnalyzer/1.0")

	resp, err := f.Client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, fmt.Errorf("request timed out")
		}
		return nil, nil, err
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(strings.ToLower(contentType), "text/html") {
		return nil, nil, fmt.Errorf("not an HTML page (Content-Type: %s)", contentType)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize+1))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}
	if len(body) > maxBodySize {
		return nil, nil, fmt.Errorf("response too large (exceeds 5MB)")
	}

	return body, resp.Request.URL, nil
}
