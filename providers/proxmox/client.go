// SPDX-License-Identifier: Apache-2.0

package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// Client represents a Proxmox VE API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	ticket     string
	csrf       string
	username   string
	realm      string
	config     *config.ProxmoxConfig
	logger     logger.Logger
	retryer    *retry.Retryer
}

// AuthResponse represents authentication response
type AuthResponse struct {
	Data struct {
		Ticket            string `json:"ticket"`
		CSRFToken         string `json:"CSRFPreventionToken"`
		Username          string `json:"username"`
		ClusterName       string `json:"clustername"`
		CertificateDigest string `json:"cap"`
	} `json:"data"`
}

// APIResponse represents a generic API response
type APIResponse struct {
	Data json.RawMessage `json:"data"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Errors []string `json:"errors"`
}

// NewClient creates a new Proxmox VE client
func NewClient(cfg *config.ProxmoxConfig, log logger.Logger) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("proxmox config is required")
	}

	// Build base URL
	port := cfg.Port
	if port == 0 {
		port = 8006
	}

	baseURL := fmt.Sprintf("https://%s:%d/api2/json", cfg.Host, port)

	// Create HTTP client with custom TLS config
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			// #nosec G402 -- VerifySSL is an explicit, operator-controlled
			// config option (defaults reflect the operator's own choice) for
			// environments with self-signed Proxmox certificates; it is not
			// hardcoded to skip verification.
			InsecureSkipVerify: !cfg.VerifySSL,
		},
	}

	timeout := 30 * time.Second

	// Initialize retryer with defaults
	retryConfig := &retry.RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
	retryer := retry.NewRetryer(retryConfig, log)

	// Parse username and realm from username (format: user@realm)
	username := cfg.Username
	realm := "pam" // default
	if strings.Contains(username, "@") {
		parts := strings.SplitN(username, "@", 2)
		username = parts[0]
		realm = parts[1]
	}

	client := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   timeout,
		},
		username: username,
		realm:    realm,
		config:   cfg,
		logger:   log,
		retryer:  retryer,
	}

	// Authenticate
	if err := client.authenticate(cfg.Username, cfg.Password, realm); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	log.Info("connected to Proxmox VE", "host", cfg.Host, "user", cfg.Username)

	return client, nil
}

// SetNetworkMonitor sets the network monitor for retry operations
func (c *Client) SetNetworkMonitor(monitor retry.NetworkMonitor) {
	c.retryer.SetNetworkMonitor(monitor)
}

// authenticate performs authentication and stores ticket/CSRF token
func (c *Client) authenticate(username, password, realm string) error {
	if realm == "" {
		realm = "pam"
	}

	authURL := c.baseURL + "/access/ticket"

	// Prepare form data
	data := url.Values{}
	data.Set("username", fmt.Sprintf("%s@%s", username, realm))
	data.Set("password", password)

	// Create request
	req, err := http.NewRequest("POST", authURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send auth request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("decode auth response: %w", err)
	}

	c.ticket = authResp.Data.Ticket
	c.csrf = authResp.Data.CSRFToken

	c.logger.Debug("authenticated successfully",
		"username", authResp.Data.Username,
		"cluster", authResp.Data.ClusterName)

	return nil
}

// apiRequest performs an authenticated API request
func (c *Client) apiRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	reqURL := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set authentication cookie
	req.Header.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", c.ticket))

	// Set CSRF token for non-GET requests
	if method != "GET" {
		req.Header.Set("CSRFPreventionToken", c.csrf)
	}

	// Set content type for POST/PUT
	if method == "POST" || method == "PUT" {
		if body != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}

	c.logger.Debug("api request", "method", method, "path", path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	return resp, nil
}

// ListNodes returns list of cluster nodes
func (c *Client) ListNodes(ctx context.Context) ([]Node, error) {
	resp, err := c.apiRequest(ctx, "GET", "/nodes", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list nodes failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data []Node `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return apiResp.Data, nil
}

// Node represents a Proxmox cluster node
type Node struct {
	Node           string  `json:"node"`
	Status         string  `json:"status"`
	CPU            float64 `json:"cpu"`
	MaxCPU         int     `json:"maxcpu"`
	Memory         int64   `json:"mem"`
	MaxMemory      int64   `json:"maxmem"`
	Disk           int64   `json:"disk"`
	MaxDisk        int64   `json:"maxdisk"`
	Uptime         int64   `json:"uptime"`
	Level          string  `json:"level"`
	Type           string  `json:"type"`
	SSLFingerprint string  `json:"ssl_fingerprint"`
}

// ListVMs returns list of VMs on a specific node
func (c *Client) ListVMs(ctx context.Context, node string) ([]VM, error) {
	path := fmt.Sprintf("/nodes/%s/qemu", node)

	resp, err := c.apiRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list VMs failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data []VM `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	c.logger.Info("listed VMs", "node", node, "count", len(apiResp.Data))

	return apiResp.Data, nil
}

// VM represents a Proxmox QEMU VM
type VM struct {
	VMID      int     `json:"vmid"`
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	CPU       float64 `json:"cpu"`
	CPUs      int     `json:"cpus"`
	Memory    int64   `json:"mem"`
	MaxMemory int64   `json:"maxmem"`
	Disk      int64   `json:"disk"`
	MaxDisk   int64   `json:"maxdisk"`
	Uptime    int64   `json:"uptime"`
	NetIn     int64   `json:"netin"`
	NetOut    int64   `json:"netout"`
	DiskRead  int64   `json:"diskread"`
	DiskWrite int64   `json:"diskwrite"`
	PID       int     `json:"pid"`
}

// GetVM returns details for a specific VM
func (c *Client) GetVM(ctx context.Context, node string, vmid int) (*VM, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", node, vmid)

	resp, err := c.apiRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get VM failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data VM `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &apiResp.Data, nil
}

// GetVMConfig returns VM configuration
func (c *Client) GetVMConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid)

	resp, err := c.apiRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get VM config failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return apiResp.Data, nil
}

// StopVM stops a running VM
func (c *Client) StopVM(ctx context.Context, node string, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", node, vmid)

	resp, err := c.apiRequest(ctx, "POST", path, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stop VM failed with status %d: %s", resp.StatusCode, string(body))
	}

	c.logger.Info("VM stop initiated", "node", node, "vmid", vmid)
	return nil
}

// StartVM starts a VM
func (c *Client) StartVM(ctx context.Context, node string, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/start", node, vmid)

	resp, err := c.apiRequest(ctx, "POST", path, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("start VM failed with status %d: %s", resp.StatusCode, string(body))
	}

	c.logger.Info("VM start initiated", "node", node, "vmid", vmid)
	return nil
}

// WaitForTask waits for a task to complete
func (c *Client) WaitForTask(ctx context.Context, node, upid string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			status, err := c.GetTaskStatus(ctx, node, upid)
			if err != nil {
				return err
			}

			c.logger.Debug("task status", "upid", upid, "status", status)

			if status == "stopped" {
				c.logger.Info("task completed", "upid", upid)
				return nil
			}
		}
	}
}

// GetTaskStatus returns the status of a task
func (c *Client) GetTaskStatus(ctx context.Context, node, upid string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", node, url.PathEscape(upid))

	resp, err := c.apiRequest(ctx, "GET", path, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get task status failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return apiResp.Data.Status, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	c.logger.Debug("closing Proxmox client")
	// Proxmox doesn't require explicit logout, ticket expires automatically
	return nil
}
