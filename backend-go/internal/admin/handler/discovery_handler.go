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

// GetRSSHubSettings GET /api/settings/rsshub — 读 RSSHub 实例地址（缺省回落 DefaultRSSHubBaseURL）。
func GetRSSHubSettings(c *gin.Context) {
	baseURL := service.DefaultRSSHubBaseURL
	configured := false
	if cfg, _, err := aisettings.LoadRSSHubConfig(); err == nil {
		if u, ok := cfg["rsshub_base_url"].(string); ok && strings.TrimSpace(u) != "" {
			baseURL = strings.TrimSpace(u)
			configured = true
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"rsshub_base_url": baseURL,
			"configured":      configured,
			"default":         service.DefaultRSSHubBaseURL,
		},
	})
}

// saveRSSHubSettingsRequest 保存 RSSHub 实例地址请求体。
type saveRSSHubSettingsRequest struct {
	RSSHubBaseURL string `json:"rsshub_base_url"`
}

// SaveRSSHubSettings POST /api/settings/rsshub — 写 RSSHub 实例地址（空串=恢复默认）。
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
	c.JSON(http.StatusOK, gin.H{"success": true})
}
