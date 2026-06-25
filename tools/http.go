package tools

import (
	"context"
	"io"
	"net/http"
	"time"
)

type requestBuilder interface {
	NewRequestWithContext(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error)
}

type defaultRequestBuilderProvider struct{}

func (d *defaultRequestBuilderProvider) NewRequestWithContext(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, method, url, body)
}

var requestBuilderProvider requestBuilder = &defaultRequestBuilderProvider{}

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type httpClientFactory func(timeout time.Duration) httpClient

var newHTTPClient httpClientFactory = func(timeout time.Duration) httpClient {
	return &http.Client{Timeout: timeout}
}
