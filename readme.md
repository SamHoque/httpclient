# HTTP Client Library

A Go HTTP client library with built-in caching and automatic background updates.

## 🚀 Quick Start (2 minutes)

### Basic HTTP Client
```go
// 1. Import the package
import "github.com/samhoque/httpclient"

// 2. Create a client
client := httpclient.NewClient(
    "https://api.example.com",
    httpclient.WithTimeout(10*time.Second),
)

// 3. Make requests
resp, err := client.Get(context.Background(), "/users")
resp, err := client.Post(context.Background(), "/users", user)
```

### Cached HTTP Client
```go
// 1. Create a cached client
client := httpclient.NewCachedClient("https://api.example.com")
defer client.Stop()

// 2. Define your data structure
var users []struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

// 3. Setup automatic cache updates
err := client.SetupCachedEndpoint(
    context.Background(),
    httpclient.CacheConfig{
        Path:       "/users",
        CronSpec:   "*/15 * * * *",  // Every 15 minutes
        Expiration: 20 * time.Minute,
    },
    &users,
)

// 4. Use the cached data
data, err := client.GetCached("/users")
```

## 📦 Installation
```bash
go get github.com/samhoque/httpclient
```

## 🔥 Common Use Cases

### 1. API Client with Authentication
```go
client := httpclient.NewClient(
    "https://api.example.com",
    httpclient.WithHeader("Authorization", "Bearer token"),
    httpclient.WithTimeout(5*time.Second),
)
```

### 2. Cached API Data with Auto-Updates
```go
client := httpclient.NewCachedClient("https://api.example.com")
defer client.Stop()

var prices []PriceData
err := client.SetupCachedEndpoint(
    context.Background(),
    httpclient.CacheConfig{
        Path:       "/prices",
        CronSpec:   "*/5 * * * *",  // Update every 5 minutes
        Expiration: 10 * time.Minute,
    },
    &prices,
)
```

### 3. POST Request with JSON
```go
data := struct {
    Name string `json:"name"`
}{
    Name: "John",
}
resp, err := client.Post(ctx, "/users", data)
```

## ⚡️ Features At a Glance

### Basic Client
- ✅ Automatic JSON encoding/decoding
- ✅ Custom headers and timeouts
- ✅ Context support
- ✅ Clean, fluent API

### Cached Client
- ✅ Automatic background updates
- ✅ In-memory caching
- ✅ Configurable update schedules
- ✅ Thread-safe operations

## 📝 Common Cron Patterns

```go
CacheConfig{
    CronSpec: "*/15 * * * *"  // Every 15 minutes
    CronSpec: "0 * * * *"     // Every hour
    CronSpec: "0 0 * * *"     // Every day at midnight
}
```

## 🚨 Error Handling

```go
// Basic error handling
resp, err := client.Get(ctx, "/users")
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        // Handle timeout
    }
    return err
}
defer resp.Body.Close()

// Cache error handling
data, err := client.GetCached("/users")
if err != nil {
    // If cache expired, fetch fresh data
    data, err = client.GetCachedOrFetch(ctx, "/users")
}
```

## 🔍 Advanced Usage

<details>
<summary>Multiple Cached Endpoints</summary>

```go
client := httpclient.NewCachedClient("https://api.example.com")
defer client.Stop()

var users []UserData
var posts []PostData

// Setup multiple endpoints
err := client.SetupCachedEndpoint(ctx, CacheConfig{
    Path:       "/users",
    CronSpec:   "*/15 * * * *",
}, &users)

err = client.SetupCachedEndpoint(ctx, CacheConfig{
    Path:       "/posts",
    CronSpec:   "*/30 * * * *",
}, &posts)
```
</details>

<details>
<summary>Custom Configuration</summary>

```go
client := httpclient.NewClient(
    "https://api.example.com",
    httpclient.WithTimeout(5*time.Second),
    httpclient.WithHeader("X-API-Key", "key"),
    httpclient.WithHeader("User-Agent", "MyApp/1.0"),
)
```
</details>

## ⚠️ Common Gotchas
1. Always `defer client.Stop()` for cached clients
2. Always `defer resp.Body.Close()` for responses
3. Cache expiration is separate from update schedule

## 🤝 Need Help?
- Report issues: [GitHub Issues](https://github.com/samhoque/httpclient/issues)
- Contribute: [GitHub Repository](https://github.com/samhoque/httpclient)

## Full API Reference
For complete API documentation, see our [GoDoc](https://pkg.go.dev/github.com/samhoque/httpclient).