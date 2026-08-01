package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/aisettings"
	"syntopica-backend/internal/reader/repository"
	"syntopica-backend/internal/reader/service"
	tagging "syntopica-backend/internal/tagmanagement"
)

type SaveFirecrawlSettingsRequest struct {
	Enabled          bool   `json:"enabled"`
	APIUrl           string `json:"api_url"`
	APIKey           string `json:"api_key"`
	Mode             string `json:"mode"`
	Timeout          int    `json:"timeout"`
	MaxContentLength int    `json:"max_content_length"`
}

func CrawlArticle(c *gin.Context) {
	articleID := c.Param("id")

	var article models.Article
	if err := repository.Repo.DB().First(&article, articleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Article not found",
		})
		return
	}

	var feed models.Feed
	if err := repository.Repo.DB().First(&feed, article.FeedID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Feed not found",
		})
		return
	}

	if !feed.FirecrawlEnabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Firecrawl not enabled for this feed",
		})
		return
	}

	config, err := service.GetFirecrawlConfig()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if !config.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Firecrawl is not enabled globally",
		})
		return
	}

	firecrawlService := service.NewFallbackCrawler(
		service.NewReadabilityCrawler(),
		service.NewFirecrawlService(config),
	)

	article.FirecrawlStatus = "processing"
	repository.Repo.DB().Save(&article)

	result, err := firecrawlService.ScrapePage(c.Request.Context(), article.Link)
	if err != nil {
		article.FirecrawlStatus = "failed"
		article.FirecrawlError = err.Error()
		repository.Repo.DB().Save(&article)

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	article.FirecrawlStatus = "completed"
	article.FirecrawlContent = result.Markdown
	article.FirecrawlError = ""
	article.SummaryStatus = "incomplete"
	now := time.Now()
	article.FirecrawlCrawledAt = &now
	repository.Repo.DB().Save(&article)
	if feed.TaggingEnabled {
		_ = tagging.NewTagJobQueue(repository.Repo.DB()).Enqueue(tagging.TagJobRequest{
			ArticleID:    article.ID,
			FeedName:     feed.Title,
			CategoryName: tagging.FeedCategoryName(feed),
			ForceRetag:   true,
			Reason:       "manual_firecrawl_completed",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"firecrawl_content": result.Markdown,
			"firecrawl_status":  "completed",
			"summary_status":    "incomplete",
		},
	})
}

func EnableFeedFirecrawl(c *gin.Context) {
	feedID := c.Param("id")

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	var feed models.Feed
	if err := repository.Repo.DB().First(&feed, feedID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Feed not found",
		})
		return
	}

	feed.FirecrawlEnabled = req.Enabled
	repository.Repo.DB().Save(&feed)

	if req.Enabled {
		repository.Repo.DB().Model(&models.Article{}).
			Where("feed_id = ?", feed.ID).
			Where("(firecrawl_content IS NULL OR firecrawl_content = '') AND firecrawl_status <> ?", "processing").
			Updates(map[string]interface{}{
				"firecrawl_status": "pending",
				"firecrawl_error":  "",
			})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"firecrawl_enabled": feed.FirecrawlEnabled,
		},
	})
}

func GetFirecrawlStatus(c *gin.Context) {
	config, err := service.GetFirecrawlConfig()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"enabled": false,
				"error":   err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":            config.Enabled,
			"api_url":            config.APIUrl,
			"mode":               config.Mode,
			"timeout":            config.Timeout,
			"max_content_length": config.MaxContentLength,
			"api_key_configured": config.APIKey != "",
		},
	})
}

func SaveFirecrawlSettings(c *gin.Context) {
	var req SaveFirecrawlSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if req.Mode == "" {
		req.Mode = "scrape"
	}
	if req.Timeout <= 0 {
		req.Timeout = 60
	}
	if req.MaxContentLength <= 0 {
		req.MaxContentLength = 50000
	}

	configJSON, _, err := aisettings.LoadFirecrawlConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	apiKey := req.APIKey
	if apiKey == "" {
		apiKey = service.GetFirecrawlAPIKey(configJSON)
	}

	configJSON = map[string]interface{}{
		"enabled":            req.Enabled,
		"api_url":            req.APIUrl,
		"api_key":            apiKey,
		"mode":               req.Mode,
		"timeout":            req.Timeout,
		"max_content_length": req.MaxContentLength,
	}

	if err := aisettings.SaveFirecrawlConfig(configJSON, "Firecrawl configuration"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Firecrawl settings saved successfully",
		"data": gin.H{
			"enabled":            req.Enabled,
			"api_url":            req.APIUrl,
			"mode":               req.Mode,
			"timeout":            req.Timeout,
			"max_content_length": req.MaxContentLength,
			"api_key_configured": apiKey != "",
		},
	})
}
