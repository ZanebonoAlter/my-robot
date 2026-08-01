package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syntopica-backend/internal/dataenrichment/service"
)

// ── Test helpers ────────────────────────────────────────────────────────────

// newTestFinGeniusServer returns an httptest.Server + handler factory for FinGenius API.
func newTestFinGeniusServer() *httptest.Server {
	mux := http.NewServeMux()

	// POST /analyze — submit symbols for debate.
	mux.HandleFunc("POST /analyze", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		symbols, _ := req["symbols"].([]any)
		tasks := make([]map[string]any, 0, len(symbols))
		for _, s := range symbols {
			sym, _ := s.(map[string]any)
			tasks = append(tasks, map[string]any{
				"task_id":    "task-" + sym["code"].(string),
				"stock_code": sym["code"],
				"name":       sym["name"],
				"sector":     sym["sector"],
			})
		}
		resp := map[string]any{"tasks": tasks}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// GET /task/{task_id} — poll task status.
	mux.HandleFunc("GET /task/{task_id}", func(w http.ResponseWriter, r *http.Request) {
		taskID := r.PathValue("task_id")
		status := r.URL.Query().Get("status") // controlled via query param for testing
		if status == "" {
			status = "done"
		}

		result := map[string]any{
			"task_id":    taskID,
			"stock_code": "000001",
			"name":       "平安银行",
			"sector":     "金融",
			"status":     status,
		}

		if status == "done" {
			result["result"] = map[string]any{
				"stock_code":    "000001",
				"analysis_time": 1.5,
				"html_content":  "<html>辩论报告</html>",
				"name":          "平安银行",
				"sector":        "金融",
				"research":      map[string]any{"sentiment_agent": "积极看多"},
				"battle":        map[string]any{"final_decision": "bullish"},
			}
		}
		if status == "failed" {
			result["error"] = "task failed internal"
		}
		if status == "running" {
			result["progress"] = map[string]any{"current_agent": "risk_agent", "done": 2, "total": 6}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// GET /health — liveness check.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		fail := r.URL.Query().Get("fail")
		if fail == "true" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	return httptest.NewServer(mux)
}

// newTestFinGeniusClient creates a client pointing at a test server with short timeouts.
func newTestFinGeniusClient(serverURL string) *service.FinGeniusHTTPClient {
	return service.NewFinGeniusHTTPClientWithConfig(service.FingeniusConfig{
		BaseURL:      serverURL,
		APIKey:       "test-key",
		Timeout:      2 * time.Second,
		PollInterval: 10 * time.Millisecond,
		MaxWait:      500 * time.Millisecond,
	})
}

// ── Submit ──────────────────────────────────────────────────────────────────

func TestFinGeniusClient_Submit_Success(t *testing.T) {
	srv := newTestFinGeniusServer()
	defer srv.Close()

	client := newTestFinGeniusClient(srv.URL)
	symbols := []service.DebateSymbol{
		{Code: "000001", Name: "平安银行", Sector: "金融"},
		{Code: "600519", Name: "贵州茅台", Sector: "消费"},
	}

	tasks, err := client.Submit(context.Background(), symbols)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("tasks count: want 2, got %d", len(tasks))
	}
	if tasks[0].TaskID != "task-000001" {
		t.Fatalf("task[0].task_id: want task-000001, got %s", tasks[0].TaskID)
	}
	if tasks[0].StockCode != "000001" {
		t.Fatalf("task[0].stock_code: want 000001, got %s", tasks[0].StockCode)
	}
	if tasks[0].Name != "平安银行" {
		t.Fatalf("task[0].name: want 平安银行, got %s", tasks[0].Name)
	}
	if tasks[0].Sector != "金融" {
		t.Fatalf("task[0].sector: want 金融, got %s", tasks[0].Sector)
	}
	if tasks[1].TaskID != "task-600519" {
		t.Fatalf("task[1].task_id: want task-600519, got %s", tasks[1].TaskID)
	}
}

func TestFinGeniusClient_Submit_ServerError(t *testing.T) {
	// No server running — will get connection refused.
	client := service.NewFinGeniusHTTPClientWithConfig(service.FingeniusConfig{
		BaseURL:      "http://127.0.0.1:1", // non-routable
		Timeout:      100 * time.Millisecond,
		PollInterval: 10 * time.Millisecond,
		MaxWait:      500 * time.Millisecond,
	})

	_, err := client.Submit(context.Background(), []service.DebateSymbol{{Code: "000001", Name: "测试", Sector: "测试"}})
	if err == nil {
		t.Fatal("Submit should return error when server is unreachable")
	}
}

// ── GetTask ─────────────────────────────────────────────────────────────────

func TestFinGeniusClient_GetTask_Done(t *testing.T) {
	srv := newTestFinGeniusServer()
	defer srv.Close()

	client := newTestFinGeniusClient(srv.URL)
	result, err := client.GetTask(context.Background(), "task-000001")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}

	if result.Status != "done" {
		t.Fatalf("status: want done, got %s", result.Status)
	}
	if result.TaskID != "task-000001" {
		t.Fatalf("task_id: want task-000001, got %s", result.TaskID)
	}
	if result.Result == nil {
		t.Fatal("result should not be nil for done task")
	}
	if result.Result.HTMLContent != "<html>辩论报告</html>" {
		t.Fatalf("html_content mismatch: got %s", result.Result.HTMLContent)
	}
	if result.Result.Research == nil {
		t.Fatal("research should not be nil")
	}
	if result.Result.Battle == nil {
		t.Fatal("battle should not be nil")
	}
}

func TestFinGeniusClient_GetTask_Running(t *testing.T) {
	srv := newTestFinGeniusServer()
	defer srv.Close()

	ctx := context.Background()
	srvURL := srv.URL + "/task/task-000001?status=running"
	req, _ := http.NewRequestWithContext(ctx, "GET", srvURL, nil)
	resp, _ := http.DefaultClient.Do(req)
	var result service.DebateTaskResult
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	if result.Status != "running" {
		t.Fatalf("status: want running, got %s", result.Status)
	}
	if result.Progress == nil {
		t.Fatal("progress should not be nil for running task")
	}
	if result.Progress.CurrentAgent != "risk_agent" {
		t.Fatalf("current_agent: want risk_agent, got %s", result.Progress.CurrentAgent)
	}
}

func TestFinGeniusClient_GetTask_Failed(t *testing.T) {
	srv := newTestFinGeniusServer()
	defer srv.Close()

	ctx := context.Background()
	srvURL := srv.URL + "/task/task-000001?status=failed"
	req, _ := http.NewRequestWithContext(ctx, "GET", srvURL, nil)
	resp, _ := http.DefaultClient.Do(req)
	var result service.DebateTaskResult
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	if result.Status != "failed" {
		t.Fatalf("status: want failed, got %s", result.Status)
	}
	if result.Error != "task failed internal" {
		t.Fatalf("error: want 'task failed internal', got %s", result.Error)
	}
}

// ── PollTask ────────────────────────────────────────────────────────────────

func TestFinGeniusClient_PollTask_SuccessAfterRunning(t *testing.T) {
	// Server that returns "running" on first call, "done" on second call.
	var callCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		status := "running"
		if callCount >= 2 {
			status = "done"
		}
		result := map[string]any{
			"task_id": "poll-task",
			"status":  status,
		}
		if status == "done" {
			result["result"] = map[string]any{
				"stock_code":   "000001",
				"html_content": "done result",
				"research":     map[string]any{},
				"battle":       map[string]any{},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer srv.Close()

	client := service.NewFinGeniusHTTPClientWithConfig(service.FingeniusConfig{
		BaseURL:      srv.URL,
		Timeout:      2 * time.Second,
		PollInterval: 10 * time.Millisecond,
		MaxWait:      500 * time.Millisecond,
	})
	result, err := client.PollTask(context.Background(), "task-poll")
	if err != nil {
		t.Fatalf("PollTask: %v", err)
	}
	if result.Status != "done" {
		t.Fatalf("status: want done, got %s", result.Status)
	}
	if result.Result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestFinGeniusClient_PollTask_Timeout(t *testing.T) {
	// Server that always returns running → timeout.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]any{
			"task_id": "stuck-task",
			"status":  "running",
			"progress": map[string]any{
				"current_agent": "sentiment_agent",
				"done":          1,
				"total":         6,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer srv.Close()

	client := service.NewFinGeniusHTTPClientWithConfig(service.FingeniusConfig{
		BaseURL:      srv.URL,
		Timeout:      2 * time.Second,
		PollInterval: 50 * time.Millisecond,
		MaxWait:      200 * time.Millisecond,
	})

	_, err := client.PollTask(context.Background(), "task-stuck")
	if err == nil {
		t.Fatal("PollTask should timeout when task never completes")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error should mention 'timed out', got: %v", err)
	}
}

func TestFinGeniusClient_PollTask_FailedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]any{
			"task_id": "fail-task",
			"status":  "failed",
			"error":   "internal error",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer srv.Close()

	client := service.NewFinGeniusHTTPClientWithConfig(service.FingeniusConfig{
		BaseURL:      srv.URL,
		Timeout:      2 * time.Second,
		PollInterval: 10 * time.Millisecond,
		MaxWait:      500 * time.Millisecond,
	})

	result, err := client.PollTask(context.Background(), "task-fail")
	if err == nil {
		t.Fatal("PollTask should return error when task failed")
	}
	// The result should still be returned alongside the error.
	if result == nil {
		t.Fatal("result should not be nil when task failed")
	}
	if result.Status != "failed" {
		t.Fatalf("status: want failed, got %s", result.Status)
	}
}

// ── Health ──────────────────────────────────────────────────────────────────

func TestFinGeniusClient_Health_OK(t *testing.T) {
	srv := newTestFinGeniusServer()
	defer srv.Close()

	client := newTestFinGeniusClient(srv.URL)
	err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: unexpected error: %v", err)
	}
}

func TestFinGeniusClient_Health_ServerError(t *testing.T) {
	// Server that always returns 500 on /health.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := service.NewFinGeniusHTTPClientWithConfig(service.FingeniusConfig{
		BaseURL:      srv.URL,
		Timeout:      2 * time.Second,
		PollInterval: 10 * time.Millisecond,
		MaxWait:      500 * time.Millisecond,
	})

	err := client.Health(context.Background())
	if err == nil {
		t.Fatal("Health should return error on 500")
	}
}

// ── Integration: Submit → Poll → Done ──────────────────────────────────────

func TestFinGeniusClient_SubmitThenPollDone(t *testing.T) {
	srv := newTestFinGeniusServer()
	defer srv.Close()

	client := newTestFinGeniusClient(srv.URL)

	// Submit
	tasks, err := client.Submit(context.Background(), []service.DebateSymbol{
		{Code: "000001", Name: "平安银行", Sector: "金融"},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks: want 1, got %d", len(tasks))
	}

	// Poll (server returns done immediately)
	result, err := client.PollTask(context.Background(), tasks[0].TaskID)
	if err != nil {
		t.Fatalf("PollTask: %v", err)
	}
	if result.Status != "done" {
		t.Fatalf("status: want done, got %s", result.Status)
	}
	if result.Result.HTMLContent != "<html>辩论报告</html>" {
		t.Fatalf("html_content mismatch")
	}
}

// ── Context cancellation ────────────────────────────────────────────────────

func TestFinGeniusClient_PollTask_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]any{"task_id": "slow-task", "status": "running"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer srv.Close()

	client := service.NewFinGeniusHTTPClientWithConfig(service.FingeniusConfig{
		BaseURL:      srv.URL,
		Timeout:      10 * time.Second,
		PollInterval: 50 * time.Millisecond,
		MaxWait:      5 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel almost immediately
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := client.PollTask(ctx, "task-slow")
	if err == nil {
		t.Fatal("PollTask should return error when context cancelled")
	}
}
