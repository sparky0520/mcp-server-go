package tools

import (
	"context"
	"io"
	"net/http"
	"time"
)

type RequestBuilder interface {
	NewRequestWithContext(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error)
}

var requestBuilderProvider requestBuilder = &defaultRequestBuildProvider{}

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type httpClientFactory func(timeout time.Duration) httpClient

var newHTTPClient httpClientFactory = func(timeout time.Duration) httpClient {
	return &http.Client{Timeout: timeout}
}
