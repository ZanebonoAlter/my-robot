package repository

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"syntopica-backend/internal/models"
)

// PersistentTopicConfig holds the runtime-tunable parameters that govern
// assignment, lifecycle and backfill. Loaded from ai_settings with the
// defaults baked into the migration seed.
type PersistentTopicConfig struct {
	MatchThreshold   float64 // persistent_topic_match_threshold
	UpgradeThreshold int     // persistent_topic_upgrade_threshold
	DecayWindow      int     // persistent_topic_decay_window (days)
	ClusterThreshold float64 // persistent_topic_cluster_threshold
}

// DefaultPersistentTopicConfig returns the seed defaults; used when ai_settings
// rows are absent (e.g. before migration runs, or in fresh test DBs).
//
// ClusterThreshold default is 0.28 (complete-link). Real-data diagnosis on the
// sampled production boards (108 sections / 3 boards) calibrated this:
//   - 0.30 centroid/greedy → collapsed each board to 1 mega-topic (chaining)
//   - 0.25 complete-link   → too fragmented (b1980: 47 sections → 21 topics)
//   - 0.28 complete-link   → in target range (b1974=11, b1980=15, b2197=15)
//
// See the change verification-report for the full threshold scan.
func DefaultPersistentTopicConfig() PersistentTopicConfig {
	return PersistentTopicConfig{
		MatchThreshold:   0.30,
		UpgradeThreshold: 3,
		DecayWindow:      30,
		ClusterThreshold: 0.28,
	}
}

// LoadPersistentTopicConfig reads the four persistent_topic_* keys from
// ai_settings, falling back to defaults on missing/invalid rows. Reading is
// best-effort: a malformed value never fails the report pipeline.
func LoadPersistentTopicConfig(db *gorm.DB) PersistentTopicConfig {
	cfg := DefaultPersistentTopicConfig()
	keys := []string{
		"persistent_topic_match_threshold",
		"persistent_topic_upgrade_threshold",
		"persistent_topic_decay_window",
		"persistent_topic_cluster_threshold",
	}
	var rows []models.AISettings
	if err := db.Where("key IN ?", keys).Find(&rows).Error; err != nil {
		return cfg
	}
	for _, r := range rows {
		switch r.Key {
		case "persistent_topic_match_threshold":
			if v, err := strconv.ParseFloat(r.Value, 64); err == nil {
				cfg.MatchThreshold = v
			}
		case "persistent_topic_upgrade_threshold":
			if v, err := strconv.Atoi(r.Value); err == nil {
				cfg.UpgradeThreshold = v
			}
		case "persistent_topic_decay_window":
			if v, err := strconv.Atoi(r.Value); err == nil {
				cfg.DecayWindow = v
			}
		case "persistent_topic_cluster_threshold":
			if v, err := strconv.ParseFloat(r.Value, 64); err == nil {
				cfg.ClusterThreshold = v
			}
		}
	}
	return cfg
}

// persistentTopicPalette is the colour pool for per-topic card tinting in the
// detective wall. These are warm, low-saturation tones that read as "paper
// variants" against the cork board, distinct from the status colours (which
// stay green/blue/orange/purple/grey). Indexed by hash(topicID) % len.
var persistentTopicPalette = []string{
	"#F7E7C4", // paper warm (default)
	"#E9D5C5", // rose paper
	"#D6E4E0", // sage paper
	"#E3D9E8", // lilac paper
	"#E8D9C5", // amber paper
	"#CFE0E8", // sky paper
	"#EDE0C8", // sand paper
	"#D8E3D0", // moss paper
}

// PersistentTopicColor returns a stable colour for a topic id. The colour is
// computed from a fixed palette indexed by a hash of the id, so the same topic
// always gets the same colour across renders (no client-side flicker). Used to
// tint detective-wall cards so sections of one narrative share a hue.
func PersistentTopicColor(topicID uint) string {
	if len(persistentTopicPalette) == 0 {
		return "#F7E7C4"
	}
	// FNV-1a-ish mixing; stable and dependency-free.
	h := uint64(topicID)
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	return persistentTopicPalette[h%uint64(len(persistentTopicPalette))]
}

// ListActiveTopicsByBoard returns candidate + active topics for a board. These
// are the anchors injected into ClusterTags and consulted by the assignment
// step. Archived topics are excluded — they are no longer assignable.
func (r *TopicGraphRepository) ListActiveTopicsByBoard(boardID uint) ([]BoardPersistentTopic, error) {
	var topics []BoardPersistentTopic
	err := r.db.Where("semantic_board_id = ? AND status IN ?", boardID,
		[]string{TopicStatusCandidate, TopicStatusActive}).
		Order("status ASC, last_seen_date DESC, id ASC").
		Find(&topics).Error
	if err != nil {
		return nil, fmt.Errorf("list active topics: %w", err)
	}
	return topics, nil
}

// ListAllTopicsByBoard returns every non-archived topic for a board (used by
// the lifecycle updater, which must update consecutive_hits on candidate and
// active topics alike).
func (r *TopicGraphRepository) ListAllTopicsByBoard(boardID uint) ([]BoardPersistentTopic, error) {
	var topics []BoardPersistentTopic
	err := r.db.Where("semantic_board_id = ? AND status != ?", boardID, TopicStatusArchived).
		Find(&topics).Error
	if err != nil {
		return nil, fmt.Errorf("list all topics: %w", err)
	}
	return topics, nil
}

// CreateTopic inserts a new persistent topic row and returns it with the ID set.
func (r *TopicGraphRepository) CreateTopic(tx *gorm.DB, topic *BoardPersistentTopic) error {
	if tx == nil {
		tx = r.db
	}
	if err := tx.Create(topic).Error; err != nil {
		return fmt.Errorf("create persistent topic: %w", err)
	}
	return nil
}

// SaveTopics persists updated topic rows (lifecycle transitions). Uses Updates
// on the concrete fields to avoid clobbering embedding (stored as pgvector
// string, not re-serializable via Save).
func (r *TopicGraphRepository) SaveTopics(tx *gorm.DB, topics []BoardPersistentTopic) error {
	if tx == nil {
		tx = r.db
	}
	for i := range topics {
		if err := tx.Model(&BoardPersistentTopic{}).Where("id = ?", topics[i].ID).
			Updates(map[string]interface{}{
				"status":           topics[i].Status,
				"hit_count":        topics[i].HitCount,
				"consecutive_hits": topics[i].ConsecutiveHits,
				"first_seen_date":  topics[i].FirstSeenDate,
				"last_seen_date":   topics[i].LastSeenDate,
				"label":            topics[i].Label,
				"description":      topics[i].Description,
				"updated_at":       time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("save topic %d: %w", topics[i].ID, err)
		}
	}
	return nil
}

// UpdateSectionTopicAssignment writes the three assignment columns onto an
// already-inserted section row (sections have IDs after CreateInBatches).
func (r *TopicGraphRepository) UpdateSectionTopicAssignment(tx *gorm.DB, sectionID uint, topicID *uint, distance float64, confidence string) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Model(&DailyReportSection{}).Where("id = ?", sectionID).
		Updates(map[string]interface{}{
			"persistent_topic_id":    topicID,
			"topic_match_distance":   distance,
			"topic_match_confidence": confidence,
		}).Error
}

// ListSectionsByBoardOrdered returns all sections for a board ordered by date
// then id, with their embedding and topic assignment. Used by backfill and
// relation-rebuild identity-edge logic.
func (r *TopicGraphRepository) ListSectionsByBoardOrdered(boardID uint) ([]DailyReportSection, error) {
	type row struct {
		ID                   uint
		ReportID             uint
		ClusterIndex         int
		ClusterLabel         string
		Embedding            string
		PersistentTopicID    *uint
		TopicMatchDistance   float64
		TopicMatchConfidence string
		Day                  time.Time
	}
	var rows []row
	err := r.db.Raw(`
		SELECT s.id, s.report_id, s.cluster_index, s.cluster_label, s.embedding,
		       s.persistent_topic_id, s.topic_match_distance, s.topic_match_confidence,
		       r.period_date AS day
		FROM daily_report_sections s
		JOIN board_daily_reports r ON r.id = s.report_id
		WHERE r.semantic_board_id = ?
		ORDER BY r.period_date ASC, s.id ASC
	`, boardID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list sections by board: %w", err)
	}
	// Preserve date order in returned slices; the Day field is carried on
	// CreatedAt as a lightweight carrier (sections themselves don't store date).
	secs := make([]DailyReportSection, len(rows))
	for i, rw := range rows {
		secs[i] = DailyReportSection{
			ID:                   rw.ID,
			ReportID:             rw.ReportID,
			ClusterIndex:         rw.ClusterIndex,
			ClusterLabel:         rw.ClusterLabel,
			Embedding:            rw.Embedding,
			PersistentTopicID:    rw.PersistentTopicID,
			TopicMatchDistance:   rw.TopicMatchDistance,
			TopicMatchConfidence: rw.TopicMatchConfidence,
			CreatedAt:            rw.Day,
		}
	}
	return secs, nil
}

// UpdateTopic mutates the user-editable fields of a persistent topic (label
// and/or status). This backs PATCH /api/daily-reports/topics/:id — manual
// rename, archive and reactivate. Lifecycle counters (hit_count,
// consecutive_hits, seen dates) stay owned by the assignment algorithm and
// are never touched here. status is restricted to active/archived: demoting a
// topic back to candidate is an algorithm decision, not a user one.
func (r *TopicGraphRepository) UpdateTopic(topicID uint, label *string, status *string) (*BoardPersistentTopic, error) {
	var topic BoardPersistentTopic
	if err := r.db.First(&topic, topicID).Error; err != nil {
		return nil, fmt.Errorf("load topic %d: %w", topicID, err)
	}
	updates := map[string]interface{}{"updated_at": time.Now()}
	if label != nil {
		if trimmed := strings.TrimSpace(*label); trimmed != "" {
			updates["label"] = trimmed
		}
	}
	if status != nil && *status != "" {
		switch *status {
		case TopicStatusActive, TopicStatusArchived:
			if *status == TopicStatusActive && topic.Status == TopicStatusCandidate {
				threshold := LoadPersistentTopicConfig(r.db).UpgradeThreshold
				if topic.ConsecutiveHits < threshold {
					return nil, fmt.Errorf("话题需连续出现至少 %d 天后才能人工确认（当前 %d 天）", threshold, topic.ConsecutiveHits)
				}
			}
			updates["status"] = *status
		default:
			return nil, fmt.Errorf("invalid status %q (allowed: active, archived)", *status)
		}
	}
	if len(updates) == 1 { // only updated_at → nothing to do
		return &topic, nil
	}
	if err := r.db.Model(&BoardPersistentTopic{}).Where("id = ?", topicID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update topic %d: %w", topicID, err)
	}
	if err := r.db.First(&topic, topicID).Error; err != nil {
		return nil, fmt.Errorf("reload topic %d: %w", topicID, err)
	}
	return &topic, nil
}

// MergeTopics reassigns every section on the source topics to the target
// topic, archives the sources, and rebuilds board relations so identity edges
// follow the new ownership. Sources must share the target's board and must not
// equal the target. Sources are archived (not physically deleted) to preserve
// historical rows and keep the audit trail intact.
func (r *TopicGraphRepository) MergeTopics(targetTopicID uint, sourceTopicIDs []uint) (*BoardPersistentTopic, error) {
	if len(sourceTopicIDs) == 0 {
		return nil, fmt.Errorf("merge: no source topics")
	}
	// Deduplicate and drop the target id if the client redundantly included it.
	uniq := make(map[uint]bool)
	for _, sid := range sourceTopicIDs {
		if sid == targetTopicID {
			continue
		}
		uniq[sid] = true
	}
	if len(uniq) == 0 {
		return nil, fmt.Errorf("merge: source topics must differ from target")
	}
	srcIDs := make([]uint, 0, len(uniq))
	for id := range uniq {
		srcIDs = append(srcIDs, id)
	}

	var target BoardPersistentTopic
	if err := r.db.First(&target, targetTopicID).Error; err != nil {
		return nil, fmt.Errorf("load target topic %d: %w", targetTopicID, err)
	}
	var sources []BoardPersistentTopic
	if err := r.db.Where("id IN ?", srcIDs).Find(&sources).Error; err != nil {
		return nil, fmt.Errorf("load source topics: %w", err)
	}
	if len(sources) != len(srcIDs) {
		return nil, fmt.Errorf("merge: some source topics not found")
	}
	for _, s := range sources {
		if s.SemanticBoardID != target.SemanticBoardID {
			return nil, fmt.Errorf("merge: topic %d belongs to a different board", s.ID)
		}
	}

	boardID := target.SemanticBoardID
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Reassign every section on the sources to the target.
		if err := tx.Model(&DailyReportSection{}).
			Where("persistent_topic_id IN ?", srcIDs).
			Updates(map[string]interface{}{
				"persistent_topic_id":    targetTopicID,
				"topic_match_confidence": TopicConfAnchorHit,
			}).Error; err != nil {
			return fmt.Errorf("reassign sections: %w", err)
		}
		// Archive the sources (soft delete: the rows remain for audit).
		if err := tx.Model(&BoardPersistentTopic{}).
			Where("id IN ?", srcIDs).
			Updates(map[string]interface{}{
				"status":     TopicStatusArchived,
				"updated_at": time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("archive sources: %w", err)
		}
		// Rebuild relations so identity edges follow the new ownership.
		if err := RebuildBoardRelations(tx, boardID); err != nil {
			return fmt.Errorf("rebuild relations: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := r.db.First(&target, targetTopicID).Error; err != nil {
		return nil, fmt.Errorf("reload target topic: %w", err)
	}
	return &target, nil
}

// SplitTopic carves the given sections out of an existing topic into a freshly
// created topic. The new topic's embedding is the mean of the carved sections'
// embeddings (empty when none have embeddings, which leaves it unmatchable by
// findNearestTopic until the next backfill). At least one section must remain
// on the source topic — carving all sections is a rename, not a split.
func (r *TopicGraphRepository) SplitTopic(sourceTopicID uint, sectionIDs []uint, label string) (*BoardPersistentTopic, error) {
	if len(sectionIDs) == 0 {
		return nil, fmt.Errorf("split: no sections")
	}
	label = strings.TrimSpace(label)

	var source BoardPersistentTopic
	if err := r.db.First(&source, sourceTopicID).Error; err != nil {
		return nil, fmt.Errorf("load source topic %d: %w", sourceTopicID, err)
	}
	// Validate every requested section currently belongs to the source topic.
	var count int64
	if err := r.db.Model(&DailyReportSection{}).
		Where("id IN ? AND persistent_topic_id = ?", sectionIDs, sourceTopicID).
		Count(&count).Error; err != nil {
		return nil, fmt.Errorf("count carved sections: %w", err)
	}
	if int(count) != len(sectionIDs) {
		return nil, fmt.Errorf("split: some sections do not belong to topic %d", sourceTopicID)
	}
	// Refuse to empty the source — the caller should rename instead.
	var total int64
	if err := r.db.Model(&DailyReportSection{}).
		Where("persistent_topic_id = ?", sourceTopicID).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count source sections: %w", err)
	}
	if int(total) <= len(sectionIDs) {
		return nil, fmt.Errorf("split: cannot carve all sections (rename the topic instead)")
	}

	// Load carved sections (embedding for the new topic centroid) and their
	// date span (for first/last_seen).
	var carved []DailyReportSection
	if err := r.db.Select("id, report_id, embedding").Where("id IN ?", sectionIDs).Find(&carved).Error; err != nil {
		return nil, fmt.Errorf("load carved sections: %w", err)
	}
	type dateRow struct {
		Day time.Time
	}
	var dateRows []dateRow
	if err := r.db.Raw(`
		SELECT r.period_date AS day
		FROM daily_report_sections s
		JOIN board_daily_reports r ON r.id = s.report_id
		WHERE s.id IN ?
	`, sectionIDs).Scan(&dateRows).Error; err != nil {
		return nil, fmt.Errorf("load carved dates: %w", err)
	}
	var firstSeen, lastSeen time.Time
	if len(dateRows) > 0 {
		firstSeen, lastSeen = dateRows[0].Day, dateRows[0].Day
		for _, d := range dateRows {
			if d.Day.Before(firstSeen) {
				firstSeen = d.Day
			}
			if d.Day.After(lastSeen) {
				lastSeen = d.Day
			}
		}
	}

	// New topic embedding = mean of carved section embeddings.
	var embedding string
	var vecs [][]float64
	for _, s := range carved {
		if v, err := repoParsePgVector(s.Embedding); err == nil && len(v) > 0 {
			vecs = append(vecs, v)
		}
	}
	if len(vecs) > 0 {
		dim := len(vecs[0])
		mean := make([]float64, dim)
		for _, v := range vecs {
			for j := 0; j < dim; j++ {
				mean[j] += v[j]
			}
		}
		for j := 0; j < dim; j++ {
			mean[j] /= float64(len(vecs))
		}
		embedding = FloatsToPgVector(mean)
	}

	newLabel := label
	if newLabel == "" {
		newLabel = source.Label + "（拆分）"
	}

	boardID := source.SemanticBoardID
	newTopic := &BoardPersistentTopic{
		SemanticBoardID: boardID,
		Label:           newLabel,
		Embedding:       embedding,
		Status:          TopicStatusActive,
		FirstSeenDate:   firstSeen,
		LastSeenDate:    lastSeen,
		HitCount:        len(sectionIDs),
		ConsecutiveHits: len(sectionIDs),
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := r.CreateTopic(tx, newTopic); err != nil {
			return fmt.Errorf("create split topic: %w", err)
		}
		if err := tx.Model(&DailyReportSection{}).
			Where("id IN ?", sectionIDs).
			Updates(map[string]interface{}{
				"persistent_topic_id":    newTopic.ID,
				"topic_match_confidence": TopicConfAnchorHit,
			}).Error; err != nil {
			return fmt.Errorf("reassign carved sections: %w", err)
		}
		if err := RebuildBoardRelations(tx, boardID); err != nil {
			return fmt.Errorf("rebuild relations: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return newTopic, nil
}
