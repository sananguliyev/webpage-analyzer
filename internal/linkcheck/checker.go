package linkcheck

import (
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/sananguliyev/webpage-analyzer/internal/model"
)

type Checker struct {
	Client     *http.Client
	MaxWorkers int
	MaxLinks   int
}

func (c *Checker) Check(ctx context.Context, links []model.LinkResult) []model.LinkResult {
	maxLinks := c.MaxLinks
	if maxLinks <= 0 {
		maxLinks = 100
	}
	maxWorkers := c.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 10
	}

	if len(links) > maxLinks {
		links = links[:maxLinks]
	}

	results := make([]model.LinkResult, len(links))
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, link := range links {
		wg.Add(1)
		go func(idx int, l model.LinkResult) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = l
				results[idx].Error = "context cancelled"
				return
			}

			results[idx] = c.checkOne(ctx, l)
		}(i, link)
	}

	wg.Wait()
	return results
}

func (c *Checker) checkOne(ctx context.Context, link model.LinkResult) model.LinkResult {
	result := link

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, link.URL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; WebPageAnalyzer/1.0)")

	resp, err := c.Client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	resp.Body.Close()

	// HEAD returned 405 Method Not Allowed → retry with GET
	if resp.StatusCode == http.StatusMethodNotAllowed {
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, link.URL, nil)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; WebPageAnalyzer/1.0)")
		resp, err = c.Client.Do(req)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
	}

	result.StatusCode = resp.StatusCode
	result.IsAccessible = resp.StatusCode >= 200 && resp.StatusCode < 400
	return result
}
