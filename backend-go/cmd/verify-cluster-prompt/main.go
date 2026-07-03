// Command verify-cluster-prompt is a one-shot diagnostic that runs the updated
// ClusterTags prompt on the real board 1974 / 2026-06-20 tag set, with the
// board's existing topics (including the over-broad "特朗普在 G7 峰会期间的
// 盟友关系紧张" topic) injected. It prints the resulting clusters so we can
// eyeball whether the new prompt stops the LLM from (a) coining a label that
// invents context not present in the tags, and (b) packing unrelated tags into
// a broad person-based frame.
//
// Run from backend-go/:  go run ./cmd/verify-cluster-prompt
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"gorm.io/gorm"

	"syntopica-backend/internal/platform/config"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/topicgraph/repository"
	"syntopica-backend/internal/topicgraph/service"
)

// Tag IDs from board 1974 report 52 (2026-06-20) — the exact input the daily
// pipeline would feed to ClusterTags.
var report52TagIDs = []uint{
	5884, 16488, 16629, 16665, 16670, 16686, 16709, 16710, 16716, 16761,
	16762, 16773, 16783, 16821, 16824, 16848, 16884, 16889, 16903, 16982,
	17181, 17196, 17202, 17228, 17381, 17439, 17463, 17467, 17550, 17555,
}

const boardID = 1974

func main() {
	if err := config.LoadConfig("./configs"); err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	if config.AppConfig != nil {
		logging.Init(config.AppConfig.Log.Level, logging.FileConfig{Enabled: false})
	}
	if err := database.InitDB(config.AppConfig); err != nil {
		fmt.Fprintf(os.Stderr, "init db: %v\n", err)
		os.Exit(1)
	}
	db := database.DB
	repository.InitRepository(db)

	// Load the tag inputs (label + description + article_count) exactly as the
	// daily pipeline does. Reuse the same shape the orchestrator builds.
	tagInputs, err := loadTagInputs(db, report52TagIDs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load tags: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("loaded %d tags for board %d\n\n", len(tagInputs), boardID)

	// Replicate exactly the anchorable set the orchestrator injects into
	// ClusterTags — window-filtered + limit-capped via
	// ListAnchorableTopicsByBoard. Using today as the inspection date
	// (the most common scenario) so the prompt mirrors a real daily run.
	reportDate := repository.NormalizeReportDate(time.Now())
	topicCfg := repository.LoadPersistentTopicConfig(repository.Repo.DB())
	existing, anchorStats, err := repository.Repo.ListAnchorableTopicsByBoard(boardID, reportDate, topicCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load existing topics: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("anchorable set: active=%d candidates=%d filtered_window=%d truncated_limit=%d\n",
		anchorStats.ActiveCount, anchorStats.CandidateCount,
		anchorStats.FilteredByWindow, anchorStats.TruncatedByLimit)
	fmt.Printf("injected %d anchorable topics as durable frames:\n", len(existing))
	for _, t := range existing {
		fmt.Printf("  [id:%d] %s (status:%s, hits:%d)\n", t.ID, t.Label, t.Status, t.HitCount)
	}
	fmt.Println()

	// Run the updated ClusterTags with the real LLM.
	groups, err := service.ClusterTags(context.Background(), tagInputs, existing, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cluster: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== ClusterTags produced %d groups ===\n", len(groups))
	labelByID := make(map[uint]string, len(tagInputs))
	for _, t := range tagInputs {
		labelByID[t.ID] = t.Label
	}
	for i, g := range groups {
		matched := "new"
		if g.MatchedTopicID != nil {
			matched = fmt.Sprintf("reused topic %d", *g.MatchedTopicID)
		}
		fmt.Printf("\n[%d] %s  (%s)\n", i+1, g.GroupName, matched)
		for _, id := range g.TagIDs {
			fmt.Printf("    - [%d] %s\n", id, labelByID[id])
		}
	}
}

// loadTagInputs fetches tag rows matching ids and returns them in the TagInput
// shape the cluster pipeline consumes (label + description + article_count).
func loadTagInputs(db *gorm.DB, ids []uint) ([]repository.TagInput, error) {
	type row struct {
		ID           uint   `gorm:"column:id"`
		Label        string `gorm:"column:label"`
		Description  string `gorm:"column:description"`
		ArticleCount int    `gorm:"column:article_count"`
	}
	var rows []row
	if err := db.Table("topic_tags").
		Select(`topic_tags.id AS id, topic_tags.label AS label, topic_tags.description AS description,
			(SELECT count(*) FROM article_topic_tags art WHERE art.topic_tag_id = topic_tags.id) AS article_count`).
		Where("topic_tags.id IN ?", ids).
		Order("article_count DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	out := make([]repository.TagInput, len(rows))
	for i, r := range rows {
		out[i] = repository.TagInput{
			ID:           r.ID,
			Label:        r.Label,
			Description:  r.Description,
			ArticleCount: r.ArticleCount,
		}
	}
	return out, nil
}
