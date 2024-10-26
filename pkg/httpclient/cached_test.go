package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type TestCacheData struct {
	Value     string    `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

func setupCachedTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *CachedClient) {
	if handler == nil {
		handler = func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(TestCacheData{
				Value:     "test",
				Timestamp: time.Now(),
			})
		}
	}
	server := httptest.NewServer(handler)
	client := NewCachedClient(server.URL)
	return server, client
}

func TestNewCachedClient(t *testing.T) {
	client := NewCachedClient("https://api.example.com")
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
	defer client.Stop()

	if client.cache == nil {
		t.Error("Expected initialized cache map")
	}
	if client.cron == nil {
		t.Error("Expected initialized cron scheduler")
	}
}

func TestCachedClient_SetupCachedEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		config      CacheConfig
		handler     func(w http.ResponseWriter, r *http.Request)
		expectError bool
	}{
		{
			name: "successful setup",
			config: CacheConfig{
				Path:       "/test",
				CronSpec:   "* * * * *",
				Expiration: time.Minute,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				// Ensure proper content type is set
				w.Header().Set("Content-Type", "application/json")
				// Return a valid JSON response
				err := json.NewEncoder(w).Encode(TestCacheData{
					Value:     "test",
					Timestamp: time.Now(),
				})
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			},
			expectError: false,
		},
		{
			name: "invalid cron spec",
			config: CacheConfig{
				Path:       "/test",
				CronSpec:   "invalid",
				Expiration: time.Minute,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(TestCacheData{})
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new server for each test case
			server := httptest.NewServer(http.HandlerFunc(tt.handler))
			defer server.Close()

			// Create a new client with the test server URL
			client := NewCachedClient(server.URL)
			defer client.Stop()

			// Create a new instance of the data structure to store the result
			data := &TestCacheData{}

			// Setup the cached endpoint with proper context
			ctx := context.Background()
			err := client.SetupCachedEndpoint(ctx, tt.config, data)

			// Check if the error matches our expectation
			if (err != nil) != tt.expectError {
				t.Errorf("SetupCachedEndpoint() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			// If we don't expect an error, verify that the cache was properly set up
			if !tt.expectError {
				// Try to get the cached data
				cached, err := client.GetCached(tt.config.Path)
				if err != nil {
					t.Errorf("GetCached() error = %v", err)
					return
				}
				if cached == nil {
					t.Error("Expected cached data, got nil")
					return
				}
			}
		})
	}
}

func TestCachedClient_GetCached(t *testing.T) {
	tests := []struct {
		name          string
		setupConfig   CacheConfig
		handler       func(w http.ResponseWriter, r *http.Request)
		waitDuration  time.Duration
		expectedValue string
		expectError   bool
		errorContains string
	}{
		{
			name: "get valid cache",
			setupConfig: CacheConfig{
				Path:       "/test",
				CronSpec:   "* * * * *",
				Expiration: time.Hour,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(TestCacheData{
					Value: "cached-value",
				})
			},
			expectedValue: "cached-value",
			expectError:   false,
		},
		{
			name: "expired cache",
			setupConfig: CacheConfig{
				Path:       "/test",
				CronSpec:   "* * * * *",
				Expiration: time.Millisecond,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(TestCacheData{
					Value: "cached-value",
				})
			},
			waitDuration:  time.Millisecond * 2,
			expectError:   true,
			errorContains: "cache expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := setupCachedTestServer(t, tt.handler)
			defer server.Close()
			defer client.Stop()

			var data TestCacheData
			err := client.SetupCachedEndpoint(context.Background(), tt.setupConfig, &data)
			if err != nil {
				t.Fatalf("Failed to setup cache: %v", err)
			}

			if tt.waitDuration > 0 {
				time.Sleep(tt.waitDuration)
			}

			result, err := client.GetCached(tt.setupConfig.Path)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got error: %v", tt.expectError, err)
				return
			}

			if err != nil {
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
				return
			}

			cachedData := result.(*TestCacheData)
			if cachedData.Value != tt.expectedValue {
				t.Errorf("Expected value %s, got %s", tt.expectedValue, cachedData.Value)
			}
		})
	}
}

func TestCachedClient_Concurrency(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		time.Sleep(10 * time.Millisecond) // Simulate some work
		json.NewEncoder(w).Encode(TestCacheData{
			Value:     "test",
			Timestamp: time.Now(),
		})
	}))
	defer server.Close()

	client := NewCachedClient(server.URL)
	defer client.Stop()

	var data TestCacheData
	err := client.SetupCachedEndpoint(context.Background(), CacheConfig{
		Path:       "/test",
		CronSpec:   "*/1 * * * *",
		Expiration: time.Minute,
	}, &data)
	if err != nil {
		t.Fatalf("Failed to setup cache: %v", err)
	}

	// Test concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := client.GetCached("/test")
			if err != nil {
				t.Errorf("Concurrent read failed: %v", err)
				return
			}
			if result == nil {
				t.Error("Expected non-nil result")
				return
			}
		}()
	}
	wg.Wait()

	// Should only have made one request despite 100 reads
	if count := atomic.LoadInt32(&requestCount); count > 1 {
		t.Errorf("Expected 1 request, got %d", count)
	}
}

func TestCachedClient_UpdateCache(t *testing.T) {
	updateCount := 0
	var mu sync.Mutex
	updateChan := make(chan struct{}, 10) // Channel to track updates

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		updateCount++
		value := fmt.Sprintf("value-%d", updateCount)
		mu.Unlock()

		// Signal that an update occurred
		select {
		case updateChan <- struct{}{}:
		default:
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TestCacheData{
			Value:     value,
			Timestamp: time.Now(),
		})
	}))
	defer server.Close()

	client := NewCachedClient(server.URL)
	defer client.Stop()

	var data TestCacheData
	err := client.SetupCachedEndpoint(context.Background(), CacheConfig{
		Path:       "/test",
		CronSpec:   "@every 100ms",
		Expiration: time.Second,
	}, &data)
	if err != nil {
		t.Fatalf("Failed to setup cache: %v", err)
	}

	// Wait for at least 3 updates with timeout
	expectedUpdates := 3
	timeout := time.After(400 * time.Millisecond)
	updates := 0

	for updates < expectedUpdates {
		select {
		case <-updateChan:
			updates++
		case <-timeout:
			t.Fatalf("Timed out waiting for updates. Got %d, expected %d", updates, expectedUpdates)
			return
		}
	}

	mu.Lock()
	finalCount := updateCount
	mu.Unlock()

	if finalCount < expectedUpdates {
		t.Errorf("Expected at least %d updates, got %d", expectedUpdates, finalCount)
	}
}

func TestCachedClient_StopBehavior(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		json.NewEncoder(w).Encode(TestCacheData{
			Value:     "test",
			Timestamp: time.Now(),
		})
	}))
	defer server.Close()

	client := NewCachedClient(server.URL)

	var data TestCacheData
	err := client.SetupCachedEndpoint(context.Background(), CacheConfig{
		Path:       "/test",
		CronSpec:   "@every 100ms",
		Expiration: time.Second,
	}, &data)
	if err != nil {
		t.Fatalf("Failed to setup cache: %v", err)
	}

	// Let it make some requests
	time.Sleep(250 * time.Millisecond)
	beforeCount := atomic.LoadInt32(&requestCount)

	// Stop the client
	client.Stop()

	// Wait a bit more
	time.Sleep(250 * time.Millisecond)
	afterCount := atomic.LoadInt32(&requestCount)

	// Should not have made any new requests after stopping
	if beforeCount != afterCount {
		t.Errorf("Expected no new requests after stop, got %d new requests", afterCount-beforeCount)
	}
}

func TestCachedClient_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		handler     func(w http.ResponseWriter, r *http.Request)
		expectError bool
	}{
		{
			name: "invalid json response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"invalid json`))
			},
			expectError: true,
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
		{
			name: "empty response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := setupCachedTestServer(t, tt.handler)
			defer server.Close()
			defer client.Stop()

			var data TestCacheData
			err := client.SetupCachedEndpoint(context.Background(), CacheConfig{
				Path:       "/test",
				CronSpec:   "* * * * *",
				Expiration: time.Minute,
			}, &data)

			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got error: %v", tt.expectError, err)
			}
		})
	}
}

func TestCachedClient_SkipInitialFetch(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TestCacheData{
			Value:     "test",
			Timestamp: time.Now(),
		})
	}))
	defer server.Close()

	client := NewCachedClient(server.URL)
	defer client.Stop()

	var data TestCacheData
	err := client.SetupCachedEndpoint(context.Background(), CacheConfig{
		Path:             "/test",
		CronSpec:         "* * * * *",
		Expiration:       time.Minute,
		SkipInitialFetch: true,
	}, &data)
	if err != nil {
		t.Fatalf("Failed to setup cache: %v", err)
	}

	// Verify no initial request was made
	if count := atomic.LoadInt32(&requestCount); count != 0 {
		t.Errorf("Expected no requests, got %d", count)
	}

	// GetCachedOrFetch should trigger a fetch since cache hasn't been initialized
	_, err = client.GetCachedOrFetch(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Failed to fetch data: %v", err)
	}

	// Should now have exactly one request
	if count := atomic.LoadInt32(&requestCount); count != 1 {
		t.Errorf("Expected 1 request, got %d", count)
	}
}

func TestCachedClient_GetCachedOrFetch(t *testing.T) {
	tests := []struct {
		name          string
		setupConfig   CacheConfig
		waitDuration  time.Duration
		expectedCalls int32
		expectError   bool
	}{
		{
			name: "get from valid cache",
			setupConfig: CacheConfig{
				Path:       "/test",
				CronSpec:   "* * * * *",
				Expiration: time.Hour,
			},
			expectedCalls: 1, // Initial fetch only
		},
		{
			name: "fetch on expired cache",
			setupConfig: CacheConfig{
				Path:       "/test",
				CronSpec:   "* * * * *",
				Expiration: time.Millisecond,
			},
			waitDuration:  time.Millisecond * 2,
			expectedCalls: 2, // Initial fetch + refresh
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestCount int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&requestCount, 1)
				json.NewEncoder(w).Encode(TestCacheData{
					Value:     "test",
					Timestamp: time.Now(),
				})
			}))
			defer server.Close()

			client := NewCachedClient(server.URL)
			defer client.Stop()

			var data TestCacheData
			err := client.SetupCachedEndpoint(context.Background(), tt.setupConfig, &data)
			if err != nil {
				t.Fatalf("Failed to setup cache: %v", err)
			}

			if tt.waitDuration > 0 {
				time.Sleep(tt.waitDuration)
			}

			result, err := client.GetCachedOrFetch(context.Background(), tt.setupConfig.Path)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got error: %v", tt.expectError, err)
			}

			if atomic.LoadInt32(&requestCount) != tt.expectedCalls {
				t.Errorf("Expected %d calls, got %d", tt.expectedCalls, requestCount)
			}

			if result == nil && !tt.expectError {
				t.Error("Expected non-nil result")
			}
		})
	}
}
