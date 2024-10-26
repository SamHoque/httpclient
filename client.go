package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
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
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody bytes.Buffer // Create a buffer instead of a pointer
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody.Write(jsonBody)
	}

	// Use &reqBody.Buffer for the request body - it's safe even when empty
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &reqBody)
	if err != nil {
		return nil, err
	}

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	return c.client.Do(req)
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodPost, path, body)
}

// Put performs a PUT request
func (c *Client) Put(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodPut, path, body)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodDelete, path, nil)
}
