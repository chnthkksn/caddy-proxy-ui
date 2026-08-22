// Package caddyclient talks to Caddy's Admin API.
package caddyclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// Load replaces Caddy's entire running config. Zero-downtime, with automatic
// rollback on the Caddy side if the new config fails to apply.
func (c *Client) Load(ctx context.Context, config any) error {
	body, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/load", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build load request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("load config: caddy returned %s", resp.Status)
	}
	return nil
}

// Adapt converts Caddyfile text to Caddy's JSON config without loading it.
func (c *Client) Adapt(ctx context.Context, caddyfile string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/adapt", bytes.NewReader([]byte(caddyfile)))
	if err != nil {
		return nil, fmt.Errorf("build adapt request: %w", err)
	}
	req.Header.Set("Content-Type", "text/caddyfile")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("adapt caddyfile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("adapt caddyfile: caddy returned %s", resp.Status)
	}

	var out struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode adapt response: %w", err)
	}
	return out.Result, nil
}

// Ping reports whether Caddy's admin API is reachable, for the connectivity
// indicator. It does not validate the config content, only reachability.
func (c *Client) Ping(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/config/", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}
