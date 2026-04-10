package bootstrap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrUnauthorized is returned when the server responds with 401.
// Callers should stop the agent on this error (token revoked).
var ErrUnauthorized = errors.New("unauthorized: token invalid or revoked")

// Client talks to the provisioner bootstrap API.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: timeout},
	}
}

// Heartbeat sends the heartbeat and returns the config response.
func (c *Client) Heartbeat(req *HeartbeatRequest) (*ConfigResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal heartbeat: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/bootstrap/heartbeat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	return c.do(httpReq)
}

// GetConfig fetches the current config for a container (used at startup and fallback).
func (c *Client) GetConfig(containerID string) (*ConfigResponse, error) {
	url := fmt.Sprintf("%s/bootstrap/config/%s", c.baseURL, containerID)
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	return c.do(httpReq)
}

func (c *Client) do(req *http.Request) (*ConfigResponse, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("bootstrap API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var cfg ConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &cfg, nil
}
