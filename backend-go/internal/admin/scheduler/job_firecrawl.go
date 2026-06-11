package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"syntopica-backend/internal/admin/repository"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/platform/ws"
	content "syntopica-backend/internal/reader"
	tagging "syntopica-backend/internal/tagmanagement"
)

// FirecrawlJob runs one firecrawl cycle: claims pending crawl jobs from the
// queue and processes them sequentially.
func FirecrawlJob(queue *content.FirecrawlJobQueue, batchID string) JobFunc {
	return func(ctx context.Context) (*JobResult, error) {
		startTime := time.Now()

		config, err := content.GetFirecrawlConfig()
		if err != nil {
			return nil, fmt.Errorf("firecrawl config error: %w", err)
		}
		if !config.Enabled {
			return &JobResult{
				Data:    map[string]interface{}{},
				Summary: "firecrawl disabled",
			}, nil
		}

		firecrawlService := content.NewFirecrawlService(config)

		jobs, err := queue.Claim(50, firecrawlLeaseDuration(config))
		if err != nil {
			return nil, fmt.Errorf("claim error: %w", err)
		}

		if len(jobs) == 0 {
			return &JobResult{
				Data:    map[string]interface{}{},
				Summary: "no jobs to process",
			}, nil
		}

		// We need to broadcast progress - use a local counter
		var processingCount int32

		broadcastFirecrawlProgress(batchID, "processing", len(jobs), 0, 0, nil, &processingCount)

		logging.Infof("[Firecrawl] Starting sequential processing of %d jobs (concurrency=1)", len(jobs))

		completed := 0
		failed := 0

		for i := range jobs {
			job := jobs[i]

			var art models.Article
			if err := repository.Repo.DB().Omit("tag_count", "relevance_score").First(&art, job.ArticleID).Error; err != nil {
				failed++
				_ = queue.MarkFailed(job, err.Error(), time.Minute)
				continue
			}

			var feed models.Feed
			if err := repository.Repo.DB().First(&feed, art.FeedID).Error; err != nil {
				failed++
				repository.Repo.DB().Model(&art).Updates(map[string]interface{}{
					"firecrawl_status": "failed",
					"firecrawl_error":  err.Error(),
				})
				broadcastFirecrawlProgress(batchID, "processing", len(jobs), completed, failed, &ws.FirecrawlArticleProgress{
					ID:     art.ID,
					Title:  art.Title,
					Status: "failed",
					Error:  err.Error(),
				}, &processingCount)
				_ = queue.MarkFailed(job, err.Error(), time.Minute)
				continue
			}

			repository.Repo.DB().Model(&art).Update("firecrawl_status", "processing")

			broadcastFirecrawlProgress(batchID, "processing", len(jobs), completed, failed, &ws.FirecrawlArticleProgress{
				ID:     art.ID,
				Title:  art.Title,
				Status: "processing",
			}, &processingCount)

			result, crawlErr := firecrawlService.ScrapePage(context.Background(), art.Link)
			if crawlErr != nil {
				failed++
				repository.Repo.DB().Model(&art).Updates(map[string]interface{}{
					"firecrawl_status": "failed",
					"firecrawl_error":  crawlErr.Error(),
				})
				_ = queue.MarkFailed(job, crawlErr.Error(), firecrawlFailureBackoff(job.AttemptCount))
				broadcastFirecrawlProgress(batchID, "processing", len(jobs), completed, failed, &ws.FirecrawlArticleProgress{
					ID:     art.ID,
					Title:  art.Title,
					Status: "failed",
					Error:  crawlErr.Error(),
				}, &processingCount)
				logging.Errorf("[Firecrawl] Failed to crawl %s: %v", art.Link, crawlErr)
				continue
			}

			now := time.Now()
			updates := map[string]interface{}{
				"firecrawl_status":     "completed",
				"firecrawl_content":    result.Data.Markdown,
				"firecrawl_crawled_at": now,
			}
			if feed.ArticleSummaryEnabled {
				updates["summary_status"] = "incomplete"
			}
			repository.Repo.DB().Model(&art).Updates(updates)

			if feed.TaggingEnabled {
				if err := tagging.NewTagJobQueue(repository.Repo.DB()).Enqueue(tagging.TagJobRequest{
					ArticleID:    art.ID,
					FeedName:     feed.Title,
					CategoryName: tagging.FeedCategoryName(feed),
					ForceRetag:   true,
					Reason:       "firecrawl_completed",
				}); err != nil {
					failed++
					_ = queue.MarkFailed(job, err.Error(), time.Minute)
					logging.Warnf("[Firecrawl] Failed to enqueue retag for article %d after crawl: %v", art.ID, err)
					continue
				}
			}

			if err := queue.MarkCompleted(job.ID); err != nil {
				failed++
				logging.Errorf("[Firecrawl] Failed to mark job %d completed: %v", job.ID, err)
				continue
			}

			completed++
			broadcastFirecrawlProgress(batchID, "processing", len(jobs), completed, failed, &ws.FirecrawlArticleProgress{
				ID:     art.ID,
				Title:  art.Title,
				Status: "completed",
			}, &processingCount)

			// Rate limiting: 500ms between requests
			time.Sleep(500 * time.Millisecond)
		}

		duration := time.Since(startTime).Seconds()
		logging.Infof("[Firecrawl] Sequential crawl completed: %d completed, %d failed out of %d jobs in %.2fs",
			completed, failed, len(jobs), duration)

		broadcastFirecrawlProgress(batchID, "completed", len(jobs), completed, failed, nil, &processingCount)

		return &JobResult{
			Data: map[string]interface{}{
				"completed": completed,
				"failed":    failed,
				"total":     len(jobs),
				"batch_id":  batchID,
			},
			Summary: fmt.Sprintf("completed=%d failed=%d total=%d", completed, failed, len(jobs)),
		}, nil
	}
}

func firecrawlLeaseDuration(config *content.FirecrawlConfig) time.Duration {
	timeout := time.Duration(config.Timeout) * time.Second
	if timeout < 5*time.Minute {
		timeout = 5 * time.Minute
	}
	return timeout + 5*time.Minute
}

func firecrawlFailureBackoff(attempt int) time.Duration {
	if attempt <= 1 {
		return time.Minute
	}
	backoff := time.Duration(1<<minInt(attempt-1, 4)) * time.Minute
	if backoff > 30*time.Minute {
		return 30 * time.Minute
	}
	return backoff
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func broadcastFirecrawlProgress(batchID, status string, total, completed, failed int, current *ws.FirecrawlArticleProgress, processingCount *int32) {
	if processingCount != nil {
		v := completed + failed
		if v > math.MaxInt32 {
			v = math.MaxInt32
		}
		atomic.StoreInt32(processingCount, int32(v))
	}

	hub := ws.GetHub()
	msg := ws.FirecrawlProgressMessage{
		Type:      "firecrawl_progress",
		BatchID:   batchID,
		Status:    status,
		Total:     total,
		Completed: completed,
		Failed:    failed,
		Current:   current,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		logging.Warnf("[Firecrawl] Failed to marshal progress: %v", err)
		return
	}
	hub.BroadcastRaw(data)
}

// FirecrawlStatusEnricher returns a StatusDetailFunc that adds firecrawl-specific
// status fields.
func FirecrawlStatusEnricher() StatusDetailFunc {
	return func(result *JobResult) map[string]interface{} {
		return map[string]interface{}{
			"concurrency": 1,
		}
	}
}
