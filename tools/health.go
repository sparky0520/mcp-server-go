package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HealthCheckArgs struct {
	URL       string `json:"url"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type HealthCheckResult struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	LatencyMS  int64  `json:"latency_ms"`
	OK         bool   `json:"ok"`
}

func HealthCheck(ctx context.Context, args HealthCheckArgs) (HealthCheckResult, error) {
	if args.URL == "" {
		return HealthCheckResult{}, fmt.Errorf("url is required")
	}

	timeout := 3 * time.Second
	if args.TimeoutMS > 0 {
		timeout = time.Duration(args.TimeoutMS) * time.Millisecond
	}

	client := newHTTPClient(timeout)

	req, err := requestBuilderProvider.NewRequestWithContext(ctx, http.MethodGet, args.URL, nil)
	if err != nil {
		return HealthCheckResult{}, fmt.Errorf("failed to create request: %w", err)
	}

	start := time.Now()
	resp, err := client.Do(req)

	if err != nil {
		return HealthCheckResult{}, fmt.Errorf("failed to perform request: %w", err)
	}

	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	latency := time.Since(start).Milliseconds()

	return HealthCheckResult{
		URL:        args.URL,
		StatusCode: resp.StatusCode,
		LatencyMS:  latency,
		OK:         resp.StatusCode >= 200 && resp.StatusCode < 300,
	}, nil
}
