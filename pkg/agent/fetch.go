package agent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"wildgecu/pkg/provider/tool"
)

// FetchURLInput is the input for the fetch_url tool.
type FetchURLInput struct {
	URL string `json:"url" description:"The URL to fetch"`
}

// FetchURLOutput is the output for the fetch_url tool.
type FetchURLOutput struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

const maxFetchBytes = 512 * 1024 // 512 KB

var fetchURLTool = tool.NewTool("fetch_url",
	"Fetch a URL and return its text content. Useful for reading web pages, documentation, or API responses.",
	func(ctx context.Context, in FetchURLInput) (FetchURLOutput, error) {
		u := strings.TrimSpace(in.URL)
		if u == "" {
			return FetchURLOutput{}, fmt.Errorf("url is required")
		}
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			u = "https://" + u
		}

		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
		if err != nil {
			return FetchURLOutput{}, fmt.Errorf("invalid url: %w", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return FetchURLOutput{}, fmt.Errorf("fetch failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes))
		if err != nil {
			return FetchURLOutput{}, fmt.Errorf("reading body: %w", err)
		}

		return FetchURLOutput{
			Status: resp.StatusCode,
			Body:   string(body),
		}, nil
	},
)
