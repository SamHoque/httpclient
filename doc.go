// Package httpclient provides a flexible HTTP client with caching capabilities.
//
// The package offers two main types of clients:
// 1. A basic HTTP client with simplified request methods
// 2. A cached client that supports automatic background updates
//
// Basic usage:
//
//	client := httpclient.NewClient("https://api.example.com")
//	resp, err := client.Get(context.Background(), "/users")
//
// Cached client usage:
//
//	client := httpclient.NewCachedClient("https://api.example.com")
//	defer client.Stop()
package main
