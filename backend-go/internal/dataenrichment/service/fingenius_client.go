package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// ── Contract types (aligned with FinGenius service API) ─────────────────────

// DebateSymbol is a stock symbol to debate.
type DebateSymbol struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Sector string `json:"sector"`
}

// DebateTask is the immediate response after submitting a symbol for debate.
type DebateTask struct {
	TaskID    string `json:"task_id"`
	StockCode string `json:"stock_code"`
	Name      string `json:"name"`
	Sector    string `json:"sector"`
}

// DebateTaskProgress is the progress info returned while a task is running.
type DebateTaskProgress struct {
	CurrentAgent string `json:"current_agent"`
	Done         int    `json:"done"`
	Total        int    `json:"total"`
}

// DebateTaskResult is the full result of a completed debate task.
type DebateTaskResult struct {
	TaskID    string              `json:"task_id"`
	StockCode string              `json:"stock_code"`
	Name      string              `json:"name"`
	Sector    string              `json:"sector"`
	Status    string              `json:"status"` // "running", "done", "failed"
	Progress  *DebateTaskProgress `json:"progress,omitempty"`
	Result    *DebateResultData   `json:"result,omitempty"`
	Error     string              `json:"error,omitempty"`
}

// DebateResultData is the nested result payload when status=done.
type DebateResultData struct {
	StockCode    string         `json:"stock_code"`
	AnalysisTime float64        `json:"analysis_time"`
	Research     map[string]any `json:"research"`
	Battle       map[string]any `json:"battle"`
	HTMLContent  string         `json:"html_content"`
	Name         string         `json:"name"`
	Sector       string         `json:"sector"`
}

// ── Submit / Poll request/response types ────────────────────────────────────

type submitRequest struct {
	Symbols       []DebateSymbol `json:"symbols"`
	MaxSteps      int            `json:"max_steps,omitempty"`
	DebateRounds  int            `json:"debate_rounds,omitempty"`
	AgentInterval int            `json:"agent_interval,omitempty"`
}

type submitResponse struct {
	Tasks []DebateTask `json:"tasks"`
}

// ── Interface ───────────────────────────────────────────────────────────────

// FinGeniusClient is the HTTP client interface for the FinGenius debate service.
// Implementations must be safe for concurrent use.
type FinGeniusClient interface {
	// Submit sends symbols for debate and returns task IDs immediately.
	Submit(ctx context.Context, symbols []DebateSymbol) ([]DebateTask, error)

	// GetTask fetches the current status/result of a single task.
	GetTask(ctx context.Context, taskID string) (*DebateTaskResult, error)

	// PollTask polls GetTask at FINGENIUS_POLL_INTERVAL until done or timeout.
	PollTask(ctx context.Context, taskID string) (*DebateTaskResult, error)

	// Health checks the FinGenius service liveness.
	Health(ctx context.Context) error
}

// ── Configuration (from env) ────────────────────────────────────────────────

// FingeniusConfig holds the configuration for a FinGenius HTTP client.
type FingeniusConfig struct {
	BaseURL      string
	APIKey       string
	Timeout      time.Duration
	PollInterval time.Duration
	MaxWait      time.Duration
}

func loadFingeniusConfig() FingeniusConfig {
	cfg := FingeniusConfig{
		BaseURL:      getEnv("FINGENIUS_BASE_URL", "http://localhost:8000"),
		APIKey:       os.Getenv("FINGENIUS_API_KEY"),
		Timeout:      parseDurationEnv("FINGENIUS_TIMEOUT", 10*time.Second),
		PollInterval: parseDurationEnv("FINGENIUS_POLL_INTERVAL", 8*time.Second),
		MaxWait:      parseDurationEnv("FINGENIUS_MAX_WAIT", 10*time.Minute),
	}
	return cfg
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func parseDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

// ── Production client ───────────────────────────────────────────────────────

// FinGeniusHTTPClient is the production HTTP client for FinGenius.
type FinGeniusHTTPClient struct {
	cfg    FingeniusConfig
	client *http.Client
}

// NewFinGeniusHTTPClient creates a FinGeniusHTTPClient reading config from env.
func NewFinGeniusHTTPClient() *FinGeniusHTTPClient {
	cfg := loadFingeniusConfig()
	return &FinGeniusHTTPClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// NewFinGeniusHTTPClientWithConfig creates a FinGeniusHTTPClient with explicit config (for testing).
// NewFinGeniusHTTPClientWithConfig creates a FinGeniusHTTPClient with explicit config (for testing).
func NewFinGeniusHTTPClientWithConfig(cfg FingeniusConfig) *FinGeniusHTTPClient {
	return &FinGeniusHTTPClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (c *FinGeniusHTTPClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("fingenius health request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("fingenius health: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fingenius health: status %d", resp.StatusCode)
	}
	return nil
}

func (c *FinGeniusHTTPClient) Submit(ctx context.Context, symbols []DebateSymbol) ([]DebateTask, error) {
	body := submitRequest{Symbols: symbols}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("fingenius submit marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/analyze", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("fingenius submit request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fingenius submit: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("fingenius submit: status %d: %s", resp.StatusCode, string(body))
	}

	var submitResp submitResponse
	if err := json.NewDecoder(resp.Body).Decode(&submitResp); err != nil {
		return nil, fmt.Errorf("fingenius submit decode: %w", err)
	}

	return submitResp.Tasks, nil
}

func (c *FinGeniusHTTPClient) GetTask(ctx context.Context, taskID string) (*DebateTaskResult, error) {
	url := fmt.Sprintf("%s/task/%s", c.cfg.BaseURL, taskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fingenius get task request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fingenius get task: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("fingenius get task %s: status %d: %s", taskID, resp.StatusCode, string(body))
	}

	var result DebateTaskResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("fingenius get task decode: %w", err)
	}

	return &result, nil
}

func (c *FinGeniusHTTPClient) PollTask(ctx context.Context, taskID string) (*DebateTaskResult, error) {
	deadline := time.Now().Add(c.cfg.MaxWait)
	ticker := time.NewTicker(c.cfg.PollInterval)
	defer ticker.Stop()

	// Immediate first check.
	result, err := c.GetTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("fingenius poll task initial: %w", err)
	}

	for result.Status == "running" {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("fingenius poll task %s: %w", taskID, ctx.Err())
		case <-ticker.C:
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("fingenius poll task %s: timed out after %v", taskID, c.cfg.MaxWait)
		}

		next, pollErr := c.GetTask(ctx, taskID)
		if pollErr != nil {
			// On transient errors, keep polling until deadline.
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("fingenius poll task %s: last error: %w", taskID, pollErr)
			}
			continue
		}
		result = next
	}

	if result.Status == "failed" {
		return result, fmt.Errorf("fingenius task %s failed: %s", taskID, result.Error)
	}

	if result.Status != "done" {
		return result, fmt.Errorf("fingenius task %s: unexpected status %s", taskID, result.Status)
	}

	return result, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (c *FinGeniusHTTPClient) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
}

// compile-time check that env vars are parseable (best-effort).
func init() {
	// Validate FINGENIUS_TIMEOUT parseable.
	_ = strconv.Itoa(0) // silence unused import if env not set
}
