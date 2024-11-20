package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"time"

	"golang.org/x/net/publicsuffix"
)

type Client struct {
	client  *http.Client
	baseURL string
	headers map[string]string
}

type Option func(*Client)

// WithTimeout sets custom timeout
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.client.Timeout = timeout
	}
}

// WithHeader adds custom header
func WithHeader(key, value string) Option {
	return func(c *Client) {
		c.headers[key] = value
	}
}

// WithAuth enables cookie handling for authentication
func WithAuth() Option {
	return func(c *Client) {
		jar, err := cookiejar.New(&cookiejar.Options{
			PublicSuffixList: publicsuffix.List,
		})
		if err != nil {
			// In practice this error is very unlikely, but we should handle it gracefully
			jar, _ = cookiejar.New(nil)
		}
		c.client.Jar = jar
	}
}

// RequestOption defines per-request configuration
type RequestOption func(*http.Request)

// WithRequestHeader adds a header for a single request
func WithRequestHeader(key, value string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set(key, value)
	}
}

// NewClient creates a new HTTP client with default configurations
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
		headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// doRequest performs the HTTP request
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, opts ...RequestOption) (*http.Response, error) {
	var reqBody bytes.Buffer
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody.Write(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &reqBody)
	if err != nil {
		return nil, err
	}

	// Set default headers
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// Apply per-request options
	for _, opt := range opts {
		opt(req)
	}

	return c.client.Do(req)
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string, opts ...RequestOption) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil, opts...)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, path string, body interface{}, opts ...RequestOption) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodPost, path, body, opts...)
}

// Put performs a PUT request
func (c *Client) Put(ctx context.Context, path string, body interface{}, opts ...RequestOption) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodPut, path, body, opts...)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, path string, opts ...RequestOption) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodDelete, path, nil, opts...)
}
