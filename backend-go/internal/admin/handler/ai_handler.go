package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"syntopica-backend/internal/admin/repository"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/aisettings"
	"syntopica-backend/internal/platform/logging"
)

type UpsertProviderRequest struct {
	Name           string   `json:"name" binding:"required"`
	ProviderType   string   `json:"provider_type"`
	BaseURL        string   `json:"base_url" binding:"required"`
	APIKey         string   `json:"api_key"`
	Model          string   `json:"model" binding:"required"`
	Enabled        *bool    `json:"enabled"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	MaxTokens      *int     `json:"max_tokens"`
	Temperature    *float64 `json:"temperature"`
	EnableThinking *bool    `json:"enable_thinking"`
	Metadata       string   `json:"metadata"`
	ClearAPIKey    bool     `json:"clear_api_key"`
}

type UpdateRouteRequest struct {
	Name        string `json:"name"`
	Enabled     *bool  `json:"enabled"`
	Description string `json:"description"`
	ProviderIDs []uint `json:"provider_ids"`
}

func ListProviders(c *gin.Context) {
	store := airouter.NewStore(repository.Repo.DB())
	providers, err := store.ListProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	data := make([]gin.H, 0, len(providers))
	for _, provider := range providers {
		data = append(data, gin.H{
			"id":                 provider.ID,
			"name":               provider.Name,
			"provider_type":      provider.ProviderType,
			"base_url":           provider.BaseURL,
			"model":              provider.Model,
			"enabled":            provider.Enabled,
			"timeout_seconds":    provider.TimeoutSeconds,
			"max_tokens":         provider.MaxTokens,
			"temperature":        provider.Temperature,
			"enable_thinking":    provider.EnableThinking,
			"metadata":           provider.Metadata,
			"api_key_configured": strings.TrimSpace(provider.APIKey) != "",
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func UpsertProvider(c *gin.Context) {
	var req UpsertProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	provider := models.AIProvider{
		Name:           strings.TrimSpace(req.Name),
		ProviderType:   strings.TrimSpace(req.ProviderType),
		BaseURL:        strings.TrimSpace(req.BaseURL),
		APIKey:         strings.TrimSpace(req.APIKey),
		Model:          strings.TrimSpace(req.Model),
		TimeoutSeconds: req.TimeoutSeconds,
		MaxTokens:      req.MaxTokens,
		Temperature:    req.Temperature,
		EnableThinking: req.EnableThinking != nil && *req.EnableThinking,
		Metadata:       req.Metadata,
		Enabled:        req.Enabled == nil || *req.Enabled,
	}

	store := airouter.NewStore(repository.Repo.DB())
	if err := store.UpsertProvider(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"id": provider.ID}})
}

func UpdateProvider(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("provider_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid provider id"})
		return
	}

	var provider models.AIProvider
	if err := repository.Repo.DB().First(&provider, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "provider not found"})
		return
	}

	var req UpsertProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	provider.Name = strings.TrimSpace(req.Name)
	provider.ProviderType = strings.TrimSpace(req.ProviderType)
	provider.BaseURL = strings.TrimSpace(req.BaseURL)
	if req.ClearAPIKey {
		provider.APIKey = ""
	} else if strings.TrimSpace(req.APIKey) != "" {
		provider.APIKey = strings.TrimSpace(req.APIKey)
	}
	provider.Model = strings.TrimSpace(req.Model)
	provider.TimeoutSeconds = req.TimeoutSeconds
	provider.MaxTokens = req.MaxTokens
	provider.Temperature = req.Temperature
	provider.Metadata = req.Metadata
	if req.EnableThinking != nil {
		provider.EnableThinking = *req.EnableThinking
	}
	if req.Enabled != nil {
		provider.Enabled = *req.Enabled
	}

	store := airouter.NewStore(repository.Repo.DB())
	if err := store.UpsertProvider(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"id": provider.ID}})
}

func DeleteProvider(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("provider_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid provider id"})
		return
	}

	var provider models.AIProvider
	if err := repository.Repo.DB().First(&provider, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "provider not found"})
		return
	}

	var linkCount int64
	if err := repository.Repo.DB().Model(&models.AIRouteProvider{}).Where("provider_id = ?", provider.ID).Count(&linkCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if linkCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "provider is still used by one or more AI routes"})
		return
	}

	if err := repository.Repo.DB().Delete(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "provider deleted"})
}

func ListRoutes(c *gin.Context) {
	store := airouter.NewStore(repository.Repo.DB())
	routes, err := store.ListRoutes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	data := make([]gin.H, 0, len(routes))
	for _, route := range routes {
		providers := make([]gin.H, 0, len(route.RouteProviders))
		for _, link := range route.RouteProviders {
			providers = append(providers, gin.H{
				"id":          link.ID,
				"route_id":    link.RouteID,
				"provider_id": link.ProviderID,
				"priority":    link.Priority,
				"enabled":     link.Enabled,
				"provider": gin.H{
					"id":                 link.Provider.ID,
					"name":               link.Provider.Name,
					"provider_type":      link.Provider.ProviderType,
					"base_url":           link.Provider.BaseURL,
					"model":              link.Provider.Model,
					"enabled":            link.Provider.Enabled,
					"timeout_seconds":    link.Provider.TimeoutSeconds,
					"max_tokens":         link.Provider.MaxTokens,
					"temperature":        link.Provider.Temperature,
					"metadata":           link.Provider.Metadata,
					"api_key_configured": strings.TrimSpace(link.Provider.APIKey) != "",
				},
			})
		}

		data = append(data, gin.H{
			"id":              route.ID,
			"name":            route.Name,
			"capability":      route.Capability,
			"enabled":         route.Enabled,
			"strategy":        route.Strategy,
			"description":     route.Description,
			"route_providers": providers,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func UpdateRoute(c *gin.Context) {
	capability := strings.TrimSpace(c.Param("capability"))
	if capability == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "capability is required"})
		return
	}

	var req UpdateRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if len(req.ProviderIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "provider_ids is required"})
		return
	}

	route := &models.AIRoute{
		Capability:  capability,
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Strategy:    "ordered_failover",
		Enabled:     req.Enabled == nil || *req.Enabled,
	}
	if route.Name == "" {
		route.Name = airouter.DefaultRouteName
	}

	store := airouter.NewStore(repository.Repo.DB())
	if err := store.UpsertRoute(route, req.ProviderIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "route updated"})
}

func GetSettings(c *gin.Context) {
	primary, _, err := airouter.NewRouter().ResolvePrimaryProvider(airouter.CapabilitySummary)
	if err != nil {
		logging.Warnf("ai-settings: resolve primary provider failed: %v", err)
	}

	data := gin.H{}

	if primary != nil {
		data["provider_id"] = primary.ID
		data["provider_name"] = primary.Name
		data["base_url"] = primary.BaseURL
		data["model"] = primary.Model
		data["api_key_configured"] = strings.TrimSpace(primary.APIKey) != ""
	}

	var summarySetting models.AISettings
	if err := repository.Repo.DB().Where("key = ?", "summary_config").First(&summarySetting).Error; err == nil {
		var cfg map[string]interface{}
		if json.Unmarshal([]byte(summarySetting.Value), &cfg) == nil {
			if tr, ok := cfg["time_range"]; ok {
				if trNum, ok := tr.(float64); ok {
					data["time_range"] = int(trNum)
				}
			}
		}
	}

	var settings []models.AISettings
	repository.Repo.DB().Order("key ASC").Find(&settings)
	for _, s := range settings {
		if s.Key == "summary_config" || s.Key == "firecrawl_config" {
			continue
		}
		data[s.Key] = s.Value
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

type SaveSettingsRequest struct {
	DailyReportTime string `json:"daily_report_time"`
}

func SaveSettings(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if val, ok := req["daily_report_time"]; ok {
		strVal, ok := val.(string)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "daily_report_time must be a string"})
			return
		}
		if err := aisettings.SaveDailyReportTimeConfig(strVal); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

type TestConnectionRequest struct {
	BaseURL      string `json:"base_url" binding:"required"`
	APIKey       string `json:"api_key"`
	Model        string `json:"model"`
	ProviderType string `json:"provider_type"`
}

func TestConnection(c *gin.Context) {
	var req TestConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	provider := models.AIProvider{
		BaseURL:      strings.TrimSpace(req.BaseURL),
		APIKey:       strings.TrimSpace(req.APIKey),
		Model:        strings.TrimSpace(req.Model),
		ProviderType: strings.TrimSpace(req.ProviderType),
	}

	result, err := airouter.TestConnection(c.Request.Context(), provider)
	if err != nil {
		logging.Warnf("ai-test: connection test failed for %s: %v", provider.BaseURL, err)
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}

	msg := "连接成功"
	if len(result.Models) > 0 {
		msg = fmt.Sprintf("连接成功，共 %d 个可用模型", len(result.Models))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": msg,
		"data":    result,
	})
}

func upsertAISetting(key, value, description string) error {
	var existing models.AISettings
	err := repository.Repo.DB().Where("key = ?", key).First(&existing).Error
	if err == nil {
		existing.Value = value
		if description != "" {
			existing.Description = description
		}
		return repository.Repo.DB().Save(&existing).Error
	}
	if err == gorm.ErrRecordNotFound {
		return repository.Repo.DB().Create(&models.AISettings{
			Key:         key,
			Value:       value,
			Description: description,
		}).Error
	}
	return err
}
