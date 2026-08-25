package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"syntopica-backend/internal/admin/repository"
	"syntopica-backend/internal/admin/service"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/aisettings"
	"syntopica-backend/internal/platform/httpclient"
	"syntopica-backend/internal/platform/logging"
)

// ── discovery handler（preference-vector-feed-discovery）──
// catalog / recommendation / ask 端点。

// newRecommendationService 构造推荐 service（注入 airouter 供精排/问答；未配置 route 时降级）。
func newRecommendationService() *service.RecommendationService {
	return service.NewRecommendationService(repository.Repo.DB(), airouter.NewRouter(), nil)
}

// ── catalog ──

// SyncCatalog POST /api/discovery/catalog/sync — 手动触发 RSSHub 目录同步。
func SyncCatalog(c *gin.Context) {
	svc := service.NewCatalogSyncService(repository.Repo.DB(), "")
	summary, err := svc.SyncAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	// 同步完成后异步生成新路由 embedding（design D8；best-effort，不阻塞 HTTP 响应）。
	go func() {
		if embedded, embErr := svc.EmbedPendingRoutes(context.Background(), airouter.NewRouter()); embErr != nil {
			logging.Infof("catalog sync: embed pending routes failed: %v", embErr)
		} else if embedded > 0 {
			logging.Infof("catalog sync: embedded %d routes", embedded)
		}
	}()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": summary})
}

// GetCatalogStatus GET /api/discovery/catalog/status — 目录状态统计。
func GetCatalogStatus(c *gin.Context) {
	svc := service.NewCatalogSyncService(repository.Repo.DB(), "")
	st, err := svc.GetStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": st})
}

// ── recommendation ──

// GetRecommendations GET /api/discovery/recommendations?status=pending — 推荐卡片列表。
func GetRecommendations(c *gin.Context) {
	svc := newRecommendationService()
	status := c.DefaultQuery("status", "pending")
	cards, err := svc.GetRecommendations(c.Request.Context(), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": cards})
}

// RefreshRecommendations POST /api/discovery/recommendations/refresh — 换一批（粗筛+精排+幂等落库）。
func RefreshRecommendations(c *gin.Context) {
	svc := newRecommendationService()
	summary, err := svc.RefreshRecommendations(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": summary})
}

// acceptRequest 接受推荐的请求体。
type acceptRequest struct {
	CategoryID *uint             `json:"category_id"`
	Parameters map[string]string `json:"parameters"`
}

// AcceptRecommendation POST /api/discovery/recommendations/:id/accept — 接受（直订/填参验证后订阅）。
func AcceptRecommendation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	var req acceptRequest
	_ = c.ShouldBindJSON(&req) // body 可空（直订）
	svc := newRecommendationService()
	feed, err := svc.AcceptRecommendation(c.Request.Context(), uint(id), req.CategoryID, req.Parameters)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": feed, "message": "feed created"})
}

// DismissRecommendation POST /api/discovery/recommendations/:id/dismiss — 拒绝（冷却）。
func DismissRecommendation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	svc := newRecommendationService()
	if err := svc.DismissRecommendation(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "dismissed"})
}

// askRequest 问答请求体。
type askRequest struct {
	Question string `json:"question" binding:"required"`
}

// Ask POST /api/discovery/ask — 问答式即时推荐 + 种子写入。
func Ask(c *gin.Context) {
	var req askRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "question is required"})
		return
	}
	svc := newRecommendationService()
	cards, err := svc.Ask(c.Request.Context(), req.Question)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": cards})
}

// ── rsshub settings（design E）──

// GetRSSHubSettings GET /api/settings/rsshub — 读 RSSHub 实例地址（缺省回落 DefaultRSSHubBaseURL）
// 与官方文档基址 rsshub_doc_base（缺省回落 https://docs.rsshub.app，feed-param-options D4）。
func GetRSSHubSettings(c *gin.Context) {
	baseURL := service.DefaultRSSHubBaseURL
	configured := false
	if cfg, _, err := aisettings.LoadRSSHubConfig(); err == nil {
		if u, ok := cfg["rsshub_base_url"].(string); ok && strings.TrimSpace(u) != "" {
			baseURL = strings.TrimSpace(u)
			configured = true
		}
	}
	docBase, _ := aisettings.LoadRSSHubDocBaseConfig()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"rsshub_base_url":         baseURL,
			"configured":              configured,
			"default":                 service.DefaultRSSHubBaseURL,
			"rsshub_doc_base":         docBase,
			"rsshub_doc_base_default": aisettings.DefaultRSSHubDocBase(),
		},
	})
}

// saveRSSHubSettingsRequest 保存 RSSHub 实例地址请求体。
// RSSHubDocBase 为可选字段：非空时一并更新 rsshub_doc_base（feed-param-options D4）。
type saveRSSHubSettingsRequest struct {
	RSSHubBaseURL string `json:"rsshub_base_url"`
	RSSHubDocBase string `json:"rsshub_doc_base"`
}

// SaveRSSHubSettings POST /api/settings/rsshub — 写 RSSHub 实例地址（空串=恢复默认）。
// rsshub_doc_base 非空时一并写入（空串=不修改 doc_base）。
func SaveRSSHubSettings(c *gin.Context) {
	var req saveRSSHubSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}
	configJSON := map[string]interface{}{
		"rsshub_base_url": strings.TrimSpace(req.RSSHubBaseURL),
	}
	if err := aisettings.SaveRSSHubConfig(configJSON, "RSSHub instance configuration"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if docBase := strings.TrimSpace(req.RSSHubDocBase); docBase != "" {
		if err := aisettings.SaveRSSHubDocBaseConfig(docBase); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ── bocha settings（数据增强 web_search 后端 key，界面可配 + 动态读）──

// defaultBochaEndpoint 是博查通搜默认 endpoint（与 config.yaml 默认一致）。
const defaultBochaEndpoint = "https://api.bochaai.com/v1/web-search"

// maskAPIKey 脱敏：返回 key 末 4 位（长度≤4 时返回空，仅标“已配置”）。GET 回显用。
func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 4 {
		return ""
	}
	return key[len(key)-4:]
}

// GetBochaSettings GET /api/settings/bocha — 读博查配置（脱敏不回显完整 key）。
// 返回 {api_key_configured, api_key_hint(末4位), endpoint, enabled}。
func GetBochaSettings(c *gin.Context) {
	endpoint := defaultBochaEndpoint
	enabled := true
	apiKeyConfigured := false
	apiKeyHint := ""
	if cfg, _, err := aisettings.LoadBochaConfig(); err == nil && cfg != nil {
		if v, ok := cfg["api_key"].(string); ok && strings.TrimSpace(v) != "" {
			apiKeyConfigured = true
			apiKeyHint = maskAPIKey(v)
		}
		if v, ok := cfg["endpoint"].(string); ok && strings.TrimSpace(v) != "" {
			endpoint = strings.TrimSpace(v)
		}
		if v, ok := cfg["enabled"].(bool); ok {
			enabled = v
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"api_key_configured": apiKeyConfigured,
			"api_key_hint":       apiKeyHint,
			"endpoint":           endpoint,
			"enabled":            enabled,
		},
	})
}

// saveBochaSettingsRequest 保存博查配置请求体。
// Enabled 用指针：nil（未传）=保留现有值；非 nil=覆盖。避免前端漏传导致被设为 false。
// APIKey 空串=不改（保留原值）；非空才覆盖，防表单回填空覆盖掉已有 key。
type saveBochaSettingsRequest struct {
	APIKey   string `json:"api_key"`
	Endpoint string `json:"endpoint"`
	Enabled  *bool  `json:"enabled"`
}

// SaveBochaSettings POST /api/settings/bocha — 写博查配置。界面改即时生效（动态读）。
func SaveBochaSettings(c *gin.Context) {
	var req saveBochaSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	// 读现有配置，作为“不改”语义的缺省源。
	existing, _, _ := aisettings.LoadBochaConfig()
	if existing == nil {
		existing = map[string]interface{}{}
	}
	apiKey, _ := existing["api_key"].(string)
	endpoint, _ := existing["endpoint"].(string)
	enabled := true
	if v, ok := existing["enabled"].(bool); ok {
		enabled = v
	}

	// api_key：空串=不改，非空才覆盖。
	if k := strings.TrimSpace(req.APIKey); k != "" {
		apiKey = k
	}
	// endpoint：空串=不改（保留现有/default）；非空才覆盖。
	if ep := strings.TrimSpace(req.Endpoint); ep != "" {
		endpoint = ep
	}
	// enabled：指针 nil=不改；非 nil=覆盖。
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	configJSON := map[string]interface{}{
		"api_key":  apiKey,
		"endpoint": endpoint,
		"enabled":  enabled,
	}
	if err := aisettings.SaveBochaConfig(configJSON, "Bocha web-search configuration"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ── proxy settings（全局出站代理）──

// GetProxySettings GET /api/settings/proxy — 读全局出站代理地址（feed 抓取等所有外部请求）。
func GetProxySettings(c *gin.Context) {
	proxyURL := ""
	configured := false
	if cfg, _, err := aisettings.LoadProxyConfig(); err == nil {
		if u, ok := cfg["http_proxy_url"].(string); ok {
			proxyURL = strings.TrimSpace(u)
			configured = proxyURL != ""
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"http_proxy_url": proxyURL,
			"configured":     configured,
		},
	})
}

// saveProxySettingsRequest 保存全局出站代理地址请求体。
type saveProxySettingsRequest struct {
	HTTPProxyURL string `json:"http_proxy_url"`
}

// SaveProxySettings POST /api/settings/proxy — 写全局出站代理地址并即时生效。
// 空串=清除代理（恢复直连）。URL 需为 http/https/socks5，非法值返回 400。
func SaveProxySettings(c *gin.Context) {
	var req saveProxySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}
	proxyURL := strings.TrimSpace(req.HTTPProxyURL)
	if err := httpclient.SetProxy(proxyURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	configJSON := map[string]interface{}{
		"http_proxy_url": proxyURL,
	}
	if err := aisettings.SaveProxyConfig(configJSON, "Global outbound proxy configuration"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
