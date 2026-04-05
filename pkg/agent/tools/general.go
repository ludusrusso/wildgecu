package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"wildgecu/pkg/provider/tool"
)

// FetchURLTool is exported for use outside suites (e.g. bootstrap).
var FetchURLTool = fetchURLTool

// GeneralTools returns general-purpose tools: get_current_time and fetch_url.
func GeneralTools() []tool.Tool {
	return []tool.Tool{getCurrentTimeTool, fetchURLTool}
}

// --- get_current_time ---

type getTimeInput struct {
	Timezone string `json:"timezone,omitempty" description:"IANA timezone name"`
}

type getTimeOutput struct {
	Time     string `json:"time"`
	Timezone string `json:"timezone"`
}

var getCurrentTimeTool = tool.NewTool("get_current_time", "Get the current time in a given timezone",
	func(ctx context.Context, in getTimeInput) (getTimeOutput, error) {
		tz := in.Timezone
		if tz == "" {
			tz = "UTC"
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return getTimeOutput{}, fmt.Errorf("%w", err)
		}
		now := time.Now().In(loc)
		return getTimeOutput{
			Time:     now.Format(time.RFC3339),
			Timezone: tz,
		}, nil
	},
)

// --- fetch_url ---

type fetchURLInput struct {
	URL string `json:"url" description:"The URL to fetch"`
}

type fetchURLOutput struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

const maxFetchBytes = 512 * 1024 // 512 KB

var fetchURLTool = tool.NewTool("fetch_url",
	"Fetch a URL and return its text content. Useful for reading web pages, documentation, or API responses.",
	func(ctx context.Context, in fetchURLInput) (fetchURLOutput, error) {
		u := strings.TrimSpace(in.URL)
		if u == "" {
			return fetchURLOutput{}, fmt.Errorf("url is required")
		}
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			u = "https://" + u
		}

		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
		if err != nil {
			return fetchURLOutput{}, fmt.Errorf("invalid url: %w", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fetchURLOutput{}, fmt.Errorf("fetch failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes))
		if err != nil {
			return fetchURLOutput{}, fmt.Errorf("reading body: %w", err)
		}

		return fetchURLOutput{
			Status: resp.StatusCode,
			Body:   string(body),
		}, nil
	},
)
