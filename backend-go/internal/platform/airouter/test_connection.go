package airouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/httpclient"
	"syntopica-backend/internal/platform/tracing"
)

// TestResult 是连通性测试的结果。
type TestResult struct {
	Reachable bool     `json:"reachable"`
	Model     string   `json:"model"`
	Models    []string `json:"models,omitempty"`
}

// TestConnection 用 GET {base_url}/models 探测提供商是否可达，并返回可用模型列表。
// 这是最轻量的连通性验证（不发推理请求，不消耗 token）。
func TestConnection(ctx context.Context, provider models.AIProvider) (*TestResult, error) {
	ctx, span := otel.Tracer(tracing.ServiceName).Start(ctx, "airouter.TestConnection")
	defer span.End()
	timeout := time.Duration(provider.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	endpoint := strings.TrimRight(provider.BaseURL, "/") + "/models"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	if provider.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	resp, err := httpclient.New(httpclient.WithTimeout(timeout)).Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("无法连接 %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &errResp)
		msg := strings.TrimSpace(string(body))
		if errResp.Error != nil && errResp.Error.Message != "" {
			msg = errResp.Error.Message
		}
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, fmt.Errorf("提供商返回 HTTP %d: %s", resp.StatusCode, msg)
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	ids := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}

	return &TestResult{
		Reachable: true,
		Model:     provider.Model,
		Models:    ids,
	}, nil
}
