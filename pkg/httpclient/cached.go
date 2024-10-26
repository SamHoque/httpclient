package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/robfig/cron/v3"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CacheConfig defines the configuration for a cached endpoint
type CacheConfig struct {
	Path             string        // API endpoint path
	CronSpec         string        // Cron specification for updates
	Expiration       time.Duration // How long the cache is valid
	SkipInitialFetch bool          // Skip the initial fetch on startup
}

// CachedClient extends our base HTTP client with caching capabilities
type CachedClient struct {
	*Client
	cache     map[string]*CacheEntry
	cacheMux  sync.RWMutex
	cron      *cron.Cron
	stopChans map[string]chan struct{}
	ticker    map[string]*time.Ticker
}

type CacheEntry struct {
	Data       interface{}
	UpdatedAt  time.Time
	Expiration time.Duration
}

func NewCachedClient(baseURL string, opts ...Option) *CachedClient {
	return &CachedClient{
		Client:    NewClient(baseURL, opts...),
		cache:     make(map[string]*CacheEntry),
		cron:      cron.New(),
		stopChans: make(map[string]chan struct{}),
		ticker:    make(map[string]*time.Ticker),
	}
}

// SetupCachedEndpoint configures automatic updates for a specific endpoint
func (c *CachedClient) SetupCachedEndpoint(ctx context.Context, config CacheConfig, result interface{}) error {
	// Initialize the cache entry with zero time to force first fetch
	c.cacheMux.Lock()
	c.cache[config.Path] = &CacheEntry{
		Data:       result,
		Expiration: config.Expiration,
		UpdatedAt:  time.Time{}, // Zero time to force fetch on first GetCachedOrFetch
	}
	c.cacheMux.Unlock()

	stopChan := make(chan struct{})
	c.stopChans[config.Path] = stopChan

	updateFunc := func() {
		select {
		case <-stopChan:
			return
		default:
			updateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := c.updateCache(updateCtx, config.Path, result); err != nil {
				fmt.Printf("Error updating cache for %s: %v\n", config.Path, err)
			}
		}
	}

	// Only do initial fetch if not skipped
	if !config.SkipInitialFetch {
		if err := c.updateCache(ctx, config.Path, result); err != nil {
			return fmt.Errorf("initial cache update failed: %w", err)
		}
	}

	// Handle scheduling updates
	if strings.HasPrefix(config.CronSpec, "@every ") {
		durationStr := strings.TrimPrefix(config.CronSpec, "@every ")
		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			return fmt.Errorf("invalid duration in @every: %w", err)
		}

		if duration < time.Minute {
			ticker := time.NewTicker(duration)
			c.ticker[config.Path] = ticker

			go func() {
				for {
					select {
					case <-ticker.C:
						updateFunc()
					case <-stopChan:
						ticker.Stop()
						return
					}
				}
			}()
			return nil
		}
	}

	_, err := c.cron.AddFunc(config.CronSpec, updateFunc)
	if err != nil {
		return fmt.Errorf("failed to schedule cache updates: %w", err)
	}

	c.cron.Start()
	return nil
}

func (c *CachedClient) Stop() {
	// Stop all tickers
	for _, ticker := range c.ticker {
		if ticker != nil {
			ticker.Stop()
		}
	}

	// Close all stop channels
	for path := range c.stopChans {
		c.StopCacheUpdates(path)
	}

	// Stop cron scheduler
	if c.cron != nil {
		c.cron.Stop()
	}
}

// updateCache fetches fresh data from the endpoint and updates the cache
func (c *CachedClient) updateCache(ctx context.Context, path string, result interface{}) error {
	resp, err := c.Get(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.cacheMux.Lock()
	if entry, exists := c.cache[path]; exists {
		entry.Data = result
		entry.UpdatedAt = time.Now()
	}
	c.cacheMux.Unlock()

	return nil
}

// GetCachedOrFetch gets data from cache if available and not expired, otherwise fetches fresh data
func (c *CachedClient) GetCachedOrFetch(ctx context.Context, path string) (interface{}, error) {
	c.cacheMux.RLock()
	entry, exists := c.cache[path]
	c.cacheMux.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no cache entry for path: %s", path)
	}

	// Always fetch if the cache has never been updated
	// or if the cache is expired
	needsFetch := entry.UpdatedAt.IsZero() || time.Since(entry.UpdatedAt) > entry.Expiration

	if needsFetch {
		if err := c.updateCache(ctx, path, entry.Data); err != nil {
			return nil, fmt.Errorf("failed to fetch fresh data: %w", err)
		}

		// Re-get the updated cache entry
		c.cacheMux.RLock()
		entry = c.cache[path]
		c.cacheMux.RUnlock()
	}

	return entry.Data, nil
}

// GetCached retrieves data from cache if available and not expired
func (c *CachedClient) GetCached(path string) (interface{}, error) {
	c.cacheMux.RLock()
	entry, exists := c.cache[path]
	c.cacheMux.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no cache entry for path: %s", path)
	}

	if time.Since(entry.UpdatedAt) > entry.Expiration {
		return entry.Data, fmt.Errorf("cache expired for path: %s", path)
	}

	return entry.Data, nil
}

// StopCacheUpdates stops the cron job for a specific endpoint
func (c *CachedClient) StopCacheUpdates(path string) {
	if stopChan, exists := c.stopChans[path]; exists {
		close(stopChan)
		delete(c.stopChans, path)
	}
}
