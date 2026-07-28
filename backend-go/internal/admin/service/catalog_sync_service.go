package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"gorm.io/gorm"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/httpclient"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/platform/tracing"
)

// ── RSSHub 路由目录同步（design D2/D3，rsshub-route-catalog spec）──
//
// 来源：自建 RSSHub 实例 GET {rsshub_base_url}/api/namespace。
// 实测响应结构：{namespace: {routes: {path: {path,name,url,maintainers,example,parameters,description}}}}
// 同步按 content_hash diff：新增/变更入库，消失的标 gone（不物理删除）。

// DefaultRSSHubBaseURL 是 dump-sanitizer 已知的自建实例（design D2）。
const DefaultRSSHubBaseURL = "http://47.110.71.194:1200"

// CatalogSyncService 同步 RSSHub 路由目录。
type CatalogSyncService struct {
	db      *gorm.DB
	baseURL string
	// fetch 可注入：默认 HTTP GET /api/namespace；测试可替换为固定数据。
	fetch func(ctx context.Context) (map[string]json.RawMessage, error)
}

// NewCatalogSyncService 构造。baseURL 空则读 rsshub_config，配置也缺省回落默认自建实例（design E）。
func NewCatalogSyncService(db *gorm.DB, baseURL string) *CatalogSyncService {
	if baseURL == "" {
		baseURL = resolveRSSHubBaseURL(db)
	}
	s := &CatalogSyncService{db: db, baseURL: baseURL}
	s.fetch = s.httpFetchNamespace
	return s
}

// CatalogSyncSummary 描述一次目录同步的产出。
type CatalogSyncSummary struct {
	Inserted   int // 新增路由
	Updated    int // content_hash 变更的路由
	Gone       int // 目录中消失、标记 gone 的路由
	Total      int // 目录总路由数
	NewToEmbed int // 需生成 embedding 的新路由
}

// SyncAll 拉取全量目录并 diff 入库（D2）。
// 实例不可达时仅记日志并返回（不修改既有目录）。
func (s *CatalogSyncService) SyncAll(ctx context.Context) (*CatalogSyncSummary, error) {
	ctx, span := otel.Tracer(tracing.ServiceName).Start(ctx, "CatalogSyncService.SyncAll")
	defer span.End()
	raw, err := s.fetch(ctx)
	if err != nil {
		logging.Warnf("rsshub catalog sync: fetch failed, keep existing catalog: %v", err)
		return &CatalogSyncSummary{}, nil
	}

	records := flattenNamespace(raw)
	summary := &CatalogSyncSummary{Total: len(records)}

	// 取现有全部路由的 content_hash（namespace+path → {id, hash, status}）。
	existing, err := s.loadExistingRoutes(ctx)
	if err != nil {
		return nil, fmt.Errorf("load existing routes: %w", err)
	}

	seen := make(map[string]struct{}, len(records))
	for _, r := range records {
		key := r.Namespace + "|" + r.Path
		seen[key] = struct{}{}
		hash := r.contentHash()
		usable, requires := ParseRouteParameters(r.Path)
		row := models.RSSHubRoute{
			Namespace:          r.Namespace,
			Path:               r.Path,
			Name:               r.Name,
			URL:                r.URL,
			Description:        r.Description,
			Parameters:         r.ParametersJSON(),
			Example:            r.Example,
			RequiresParameters: requires,
			UsableDirectly:     usable,
			ContentHash:        hash,
			Status:             "unknown",
		}
		// content_hash 未变 → 跳过；gone 的重新出现 → 恢复 unknown。
		if cur, ok := existing[key]; ok {
			if cur.hash == hash && cur.status != "gone" {
				continue // 无变化
			}
			row.ID = cur.id
			if cur.hash == hash && cur.status == "gone" {
				row.Status = "unknown" // 复现：清除 gone
			}
			if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
				return nil, fmt.Errorf("update route %s: %w", key, err)
			}
			summary.Updated++
			continue
		}
		// 新增。
		if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, fmt.Errorf("insert route %s: %w", key, err)
		}
		summary.Inserted++
		summary.NewToEmbed++
	}

	// 消失的路由标 gone。
	for key, cur := range existing {
		if _, ok := seen[key]; ok {
			continue
		}
		if cur.status == "gone" {
			continue
		}
		if err := s.db.WithContext(ctx).Model(&models.RSSHubRoute{}).
			Where("id = ?", cur.id).
			Updates(map[string]any{"status": "gone", "updated_at": time.Now()}).Error; err != nil {
			return nil, fmt.Errorf("mark gone %s: %w", key, err)
		}
		summary.Gone++
	}

	logging.Infof("rsshub catalog sync: total=%d inserted=%d updated=%d gone=%d newEmbed=%d",
		summary.Total, summary.Inserted, summary.Updated, summary.Gone, summary.NewToEmbed)
	return summary, nil
}

// existingRoute 记录现有路由的 diff 元数据。
type existingRoute struct {
	id     uint
	hash   string
	status string
}

// loadExistingRoutes 取现有全部路由 namespace+path → existingRoute。
func (s *CatalogSyncService) loadExistingRoutes(ctx context.Context) (map[string]existingRoute, error) {
	type row struct {
		ID          uint
		Namespace   string
		Path        string
		ContentHash string
		Status      string
	}
	var rows []row
	err := s.db.WithContext(ctx).Model(&models.RSSHubRoute{}).
		Select("id, namespace, path, content_hash, status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]existingRoute, len(rows))
	for _, r := range rows {
		out[r.Namespace+"|"+r.Path] = existingRoute{id: r.ID, hash: r.ContentHash, status: r.Status}
	}
	return out, nil
}

// routeRecord 是从 /api/namespace 解析出的单条路由（中间结构）。
type routeRecord struct {
	Namespace   string
	Path        string
	Name        string
	URL         string
	Description string
	Example     string
	parameters  any // 原始 parameters（数组或对象）
}

// contentHash = sha256(namespace|path|name|description|parametersJSON)（D2 diff）。
func (r routeRecord) contentHash() string {
	raw := r.Namespace + "|" + r.Path + "|" + r.Name + "|" + r.Description + "|" + r.ParametersJSON()
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:32]
}

// ParametersJSON 序列化 parameters 为稳定 JSON 字符串；空值返回 "{}"（jsonb 列拒绝空串）。
func (r routeRecord) ParametersJSON() string {
	if r.parameters == nil {
		return "{}"
	}
	b, err := json.Marshal(r.parameters)
	if err != nil || len(b) == 0 {
		return "{}"
	}
	return string(b)
}

// flattenNamespace 把 {ns: {routes: {path: detail}}} 嵌套结构展平为 routeRecord 切片。
func flattenNamespace(raw map[string]json.RawMessage) []routeRecord {
	var out []routeRecord
	for ns, nsBody := range raw {
		// nsBody = {"routes": {path: detail}, "apiRoutes": {...}}
		var nsContainer struct {
			Routes    map[string]json.RawMessage `json:"routes"`
			APIRoutes map[string]json.RawMessage `json:"apiRoutes"`
		}
		if err := json.Unmarshal(nsBody, &nsContainer); err != nil {
			continue // 容错：跳过异常 namespace
		}
		for _, group := range []map[string]json.RawMessage{nsContainer.Routes, nsContainer.APIRoutes} {
			for _, detailRaw := range group {
				rec := parseRouteDetail(ns, detailRaw)
				if rec.Path == "" {
					continue
				}
				out = append(out, rec)
			}
		}
	}
	return out
}

// parseRouteDetail 解析单条路由 detail。
func parseRouteDetail(ns string, detailRaw json.RawMessage) routeRecord {
	var d struct {
		Path        string          `json:"path"`
		Name        string          `json:"name"`
		URL         string          `json:"url"`
		Example     string          `json:"example"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	}
	if err := json.Unmarshal(detailRaw, &d); err != nil {
		return routeRecord{}
	}
	var params any
	if len(d.Parameters) > 0 {
		_ = json.Unmarshal(d.Parameters, &params)
	}
	return routeRecord{
		Namespace:   ns,
		Path:        d.Path,
		Name:        d.Name,
		URL:         d.URL,
		Description: d.Description,
		Example:     d.Example,
		parameters:  params,
	}
}

// httpFetchNamespace 默认 fetcher：GET {baseURL}/api/namespace。
func (s *CatalogSyncService) httpFetchNamespace(ctx context.Context) (map[string]json.RawMessage, error) {
	url := strings.TrimRight(s.baseURL, "/") + "/api/namespace"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "syntopica-catalog-sync")
	client := httpclient.New(httpclient.WithTimeout(60 * time.Second))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rsshub /api/namespace returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// 响应顶层即 {namespace: {...}}（已验证）。
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse namespace response: %w", err)
	}
	return raw, nil
}

// CatalogStatus 是 GET /api/discovery/catalog/status 的响应。
type CatalogStatus struct {
	Total    int64 `json:"total"`
	Ok       int64 `json:"ok"`
	Broken   int64 `json:"broken"`
	Unknown  int64 `json:"unknown"`
	Gone     int64 `json:"gone"`
	Embedded int64 `json:"embedded"`
}

// GetStatus 返回目录状态统计。
func (s *CatalogSyncService) GetStatus(ctx context.Context) (*CatalogStatus, error) {
	ctx, span := otel.Tracer(tracing.ServiceName).Start(ctx, "CatalogSyncService.GetStatus")
	defer span.End()
	var st CatalogStatus
	if err := s.db.WithContext(ctx).Model(&models.RSSHubRoute{}).
		Select("COUNT(*) AS total").
		Scan(&st).Error; err != nil {
		return nil, err
	}
	st.Total = countStatus(s.db.WithContext(ctx), "")
	st.Ok = countStatus(s.db.WithContext(ctx), "ok")
	st.Broken = countStatus(s.db.WithContext(ctx), "broken")
	st.Unknown = countStatus(s.db.WithContext(ctx), "unknown")
	st.Gone = countStatus(s.db.WithContext(ctx), "gone")
	s.db.WithContext(ctx).Model(&models.RouteEmbedding{}).Count(&st.Embedded)
	return &st, nil
}

func countStatus(db *gorm.DB, status string) int64 {
	q := db.Model(&models.RSSHubRoute{})
	if status == "" {
		var c int64
		q.Count(&c)
		return c
	}
	var c int64
	q.Where("status = ?", status).Count(&c)
	return c
}
