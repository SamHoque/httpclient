# HTTP Client Library

A Go HTTP client library with built-in caching, authentication, and automatic background updates.

## 🚀 Quick Start (2 minutes)

### Basic HTTP Client
```go
// 1. Import the package
import "github.com/samhoque/httpclient"

// 2. Create a client
client := httpclient.NewClient(
    "https://api.example.com",
    httpclient.WithTimeout(10*time.Second),
    httpclient.WithAuth(), // Enable cookie handling
)

// 3. Make requests
// Basic request
resp, err := client.Get(context.Background(), "/users")

// Request with custom headers
resp, err = client.Get(context.Background(), "/users",
    httpclient.WithRequestHeader("X-Request-ID", "123"),
)

// POST with data
resp, err = client.Post(context.Background(), "/users", user)
```

### Authentication Example
```go
// Create client with auth enabled
client := httpclient.NewClient(
    "https://api.example.com",
    httpclient.WithAuth(),
)

// Login - cookies will be automatically saved
loginData := struct {
    Username string `json:"username"`
    Password string `json:"password"`
}{
    Username: "user@example.com",
    Password: "password123",
}
resp, err := client.Post(ctx, "/login", loginData)

// Subsequent requests will automatically include saved cookies
resp, err = client.Get(ctx, "/protected-endpoint")
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
    httpclient.WithAuth(),
    httpclient.WithTimeout(5*time.Second),
)
```

### 2. Per-Request Headers
```go
// Add custom headers for specific requests
resp, err := client.Get(ctx, "/users", 
    httpclient.WithRequestHeader("X-Custom-Header", "value"),
    httpclient.WithRequestHeader("X-Request-ID", "123"),
)

// Different headers for another request
resp, err = client.Post(ctx, "/data", data,
    httpclient.WithRequestHeader("Authorization", "Bearer token"),
)
```

### 3. Cached API Data with Auto-Updates
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

## ⚡️ Features At a Glance

### Basic Client
- ✅ Automatic JSON encoding/decoding
- ✅ Custom headers and timeouts
- ✅ Context support
- ✅ Clean, fluent API
- ✅ Per-request headers
- ✅ Cookie-based authentication

### Authentication
- ✅ Automatic cookie handling
- ✅ Session persistence
- ✅ Support for cookie-based auth flows
- ✅ Per-request header customization

### Cached Client
- ✅ Automatic background updates
- ✅ In-memory caching
- ✅ Configurable update schedules
- ✅ Thread-safe operations

## 🔧 Configuration Options

### Client Options
- `WithTimeout(duration)` - Set client timeout
- `WithHeader(key, value)` - Add default headers
- `WithAuth()` - Enable cookie handling

### Request Options
- `WithRequestHeader(key, value)` - Add headers to specific requests

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
<summary>Authentication with Custom Headers</summary>

```go
client := httpclient.NewClient(
    "https://api.example.com",
    httpclient.WithAuth(),
)

// Login with custom headers
resp, err := client.Post(ctx, "/login", loginData,
    httpclient.WithRequestHeader("X-Device-ID", "device123"),
)

// Subsequent authenticated requests
resp, err = client.Get(ctx, "/profile",
    httpclient.WithRequestHeader("X-Request-ID", "req123"),
)
```
</details>

<details>
<summary>Custom Configuration</summary>

```go
client := httpclient.NewClient(
    "https://api.example.com",
    httpclient.WithAuth(),
    httpclient.WithTimeout(5*time.Second),
    httpclient.WithHeader("X-API-Key", "key"),
)
```
</details>

## ⚠️ Common Gotchas
1. Always `defer client.Stop()` for cached clients
2. Always `defer resp.Body.Close()` for responses
3. Cache expiration is separate from update schedule
4. Cookie handling requires `WithAuth()` option

## 🤝 Need Help?
- Report issues: [GitHub Issues](https://github.com/samhoque/httpclient/issues)
- Contribute: [GitHub Repository](https://github.com/samhoque/httpclient)

## Full API Reference
For complete API documentation, see our [GoDoc](https://pkg.go.dev/github.com/samhoque/httpclient).