package service

import (
	"strings"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/tagmanagement/repository"
)

// buildArticleContextForTag queries the most recent articles associated with a tag
// and builds a context string (title + summary) for description generation.
func buildArticleContextForTag(tagID uint) string {
	type articleRow struct {
		Title       string
		Description string
	}

	var rows []articleRow
	err := repository.Repo.DB().Model(&models.ArticleTopicTag{}).
		Select("articles.title, articles.description").
		Joins("JOIN articles ON articles.id = article_topic_tags.article_id").
		Where("article_topic_tags.topic_tag_id = ?", tagID).
		Order("articles.pub_date DESC").
		Limit(3).
		Scan(&rows).Error
	if err != nil {
		logging.Warnf("description backfill: failed to query articles for tag %d: %v", tagID, err)
		return ""
	}

	if len(rows) == 0 {
		return ""
	}

	var parts []string
	for _, row := range rows {
		if row.Title != "" {
			parts = append(parts, row.Title)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	context := strings.Join(parts, "; ")
	runes := []rune(context)
	if len(runes) > 800 {
		context = string(runes[:800])
	}
	return context
}
