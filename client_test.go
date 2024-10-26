package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestData represents a test response structure
type TestData struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	server := httptest.NewServer(handler)
	client := NewClient(server.URL)
	return server, client
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		timeout time.Duration
		headers map[string]string
	}{
		{
			name:    "default configuration",
			baseURL: "https://api.example.com",
		},
		{
			name:    "with custom timeout",
			baseURL: "https://api.example.com",
			timeout: 5 * time.Second,
		},
		{
			name:    "with custom headers",
			baseURL: "https://api.example.com",
			headers: map[string]string{
				"Authorization": "Bearer token",
				"User-Agent":    "TestClient",
			},
		},
		{
			name:    "with empty baseURL",
			baseURL: "",
		},
		{
			name:    "with invalid baseURL",
			baseURL: "not-a-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []Option
			if tt.timeout > 0 {
				opts = append(opts, WithTimeout(tt.timeout))
			}
			for k, v := range tt.headers {
				opts = append(opts, WithHeader(k, v))
			}

			client := NewClient(tt.baseURL, opts...)

			if client.baseURL != tt.baseURL {
				t.Errorf("Expected baseURL %s, got %s", tt.baseURL, client.baseURL)
			}

			for k, v := range tt.headers {
				if client.headers[k] != v {
					t.Errorf("Expected header %s: %s, got %s", k, v, client.headers[k])
				}
			}
		})
	}
}

func TestClient_Get(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		handler        func(w http.ResponseWriter, r *http.Request)
		expectedStatus int
		expectedBody   *TestData // Changed to pointer to allow nil for invalid JSON cases
		expectError    bool
	}{
		{
			name: "successful request",
			path: "/test",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				json.NewEncoder(w).Encode(TestData{Message: "success", Status: "ok"})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   &TestData{Message: "success", Status: "ok"},
			expectError:    false,
		},
		{
			name: "not found",
			path: "/notfound",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   nil,
			expectError:    false,
		},
		{
			name: "server error",
			path: "/error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   nil,
			expectError:    false,
		},
		{
			name: "invalid JSON response",
			path: "/invalid",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("{invalid json"))
			},
			expectedStatus: http.StatusOK,
			expectedBody:   nil,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := setupTestServer(t, http.HandlerFunc(tt.handler))
			defer server.Close()

			var result TestData
			resp, err := client.Get(context.Background(), tt.path)

			// First check if there was an HTTP error
			if err != nil {
				if !tt.expectError {
					t.Fatalf("Unexpected error: %v", err)
				}
				return
			}
			defer resp.Body.Close()

			// Check status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			// Only try to decode JSON if we expect a valid response
			if tt.expectedBody != nil {
				err = json.NewDecoder(resp.Body).Decode(&result)
				if err != nil {
					if !tt.expectError {
						t.Fatalf("Failed to decode response: %v", err)
					}
					return
				}

				if result != *tt.expectedBody {
					t.Errorf("Expected body %v, got %v", tt.expectedBody, result)
				}
			} else if tt.expectError {
				// If we expect an error and have no expected body, try to decode anyway
				// to trigger the expected error
				err = json.NewDecoder(resp.Body).Decode(&result)
				if err == nil {
					t.Error("Expected JSON decode error, got none")
				}
			}
		})
	}
}

func TestClient_Post(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		body           interface{}
		handler        func(w http.ResponseWriter, r *http.Request)
		expectedStatus int
		expectedBody   TestData
		expectError    bool
	}{
		{
			name: "successful post",
			path: "/test",
			body: TestData{Message: "test", Status: "pending"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST request, got %s", r.Method)
				}

				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("Failed to read request body: %v", err)
				}

				var received TestData
				if err := json.Unmarshal(body, &received); err != nil {
					t.Fatalf("Failed to parse request body: %v", err)
				}

				json.NewEncoder(w).Encode(TestData{Message: "success", Status: "ok"})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   TestData{Message: "success", Status: "ok"},
		},
		{
			name: "invalid request body",
			path: "/test",
			body: make(chan int), // Cannot be marshaled to JSON
			handler: func(w http.ResponseWriter, r *http.Request) {
				t.Error("Handler should not be called")
			},
			expectError: true,
		},
		{
			name: "server error response",
			path: "/test",
			body: TestData{Message: "test", Status: "pending"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := setupTestServer(t, http.HandlerFunc(tt.handler))
			defer server.Close()

			resp, err := client.Post(context.Background(), tt.path, tt.body)
			if (err != nil) != tt.expectError {
				t.Fatalf("Expected error: %v, got error: %v", tt.expectError, err)
			}

			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
				}

				if tt.expectedStatus == http.StatusOK {
					var result TestData
					if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
						t.Fatalf("Failed to decode response: %v", err)
					}
					if result != tt.expectedBody {
						t.Errorf("Expected body %v, got %v", tt.expectedBody, result)
					}
				}
			}
		})
	}
}

func TestClient_Put(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		body           interface{}
		handler        func(w http.ResponseWriter, r *http.Request)
		expectedStatus int
		expectedBody   TestData
		expectError    bool
	}{
		{
			name: "successful put",
			path: "/test/1",
			body: TestData{Message: "update", Status: "pending"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("Expected PUT request, got %s", r.Method)
				}

				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("Failed to read request body: %v", err)
				}

				var received TestData
				if err := json.Unmarshal(body, &received); err != nil {
					t.Fatalf("Failed to parse request body: %v", err)
				}

				json.NewEncoder(w).Encode(TestData{Message: "updated", Status: "ok"})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   TestData{Message: "updated", Status: "ok"},
		},
		{
			name: "not found",
			path: "/test/999",
			body: TestData{Message: "update", Status: "pending"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectError:    false,
		},
		{
			name: "invalid request body",
			path: "/test/1",
			body: make(chan int), // Cannot be marshaled to JSON
			handler: func(w http.ResponseWriter, r *http.Request) {
				t.Error("Handler should not be called")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := setupTestServer(t, http.HandlerFunc(tt.handler))
			defer server.Close()

			resp, err := client.Put(context.Background(), tt.path, tt.body)
			if (err != nil) != tt.expectError {
				t.Fatalf("Expected error: %v, got error: %v", tt.expectError, err)
			}

			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
				}

				if tt.expectedStatus == http.StatusOK {
					var result TestData
					if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
						t.Fatalf("Failed to decode response: %v", err)
					}
					if result != tt.expectedBody {
						t.Errorf("Expected body %v, got %v", tt.expectedBody, result)
					}
				}
			}
		})
	}
}

func TestClient_Delete(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		handler        func(w http.ResponseWriter, r *http.Request)
		expectedStatus int
		expectError    bool
	}{
		{
			name: "successful delete",
			path: "/test/1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("Expected DELETE request, got %s", r.Method)
				}
				w.WriteHeader(http.StatusNoContent)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name: "not found",
			path: "/test/999",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectError:    false,
		},
		{
			name: "server error",
			path: "/test/error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := setupTestServer(t, http.HandlerFunc(tt.handler))
			defer server.Close()

			resp, err := client.Delete(context.Background(), tt.path)
			if (err != nil) != tt.expectError {
				t.Fatalf("Expected error: %v, got error: %v", tt.expectError, err)
			}

			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
				}
			}
		})
	}
}

func TestContext_Cancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(TestData{Message: "success", Status: "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Get(ctx, "/test")
	if err == nil {
		t.Error("Expected error due to context cancellation, got none")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected deadline exceeded error, got: %v", err)
	}
}

func TestClient_RequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		json.NewEncoder(w).Encode(TestData{Message: "success", Status: "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL, WithTimeout(100*time.Millisecond))

	_, err := client.Get(context.Background(), "/test")
	if err == nil {
		t.Error("Expected error due to request timeout, got none")
	}
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}
