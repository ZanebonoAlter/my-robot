package repository

import (
	"fmt"
	"time"

	"syntopica-backend/internal/platform/logging"

	"gorm.io/gorm"
)

// topicCluster is a backfill working cluster: a centroid (running mean of
// member embeddings) plus the section indices it has absorbed.
//
// memberVecs is retained so the clustering step can use COMPLETE-LINK distance
// (a candidate joins only when within ClusterThreshold of EVERY member) rather
// than running-mean centroid distance. Real-data diagnosis showed centroid
// clustering chains: board 1980 (47 sections) collapsed to 1 topic at thr=0.30
// because adjacent sections drift the centroid and pull the next in. Complete-
// link bounds a cluster's diameter, preventing the chain and yielding a topic
// count in the target 5-15 range at thr=0.25.
type topicCluster struct {
	centroid   []float64
	memberVecs [][]float64
	members    []int // indices into the section slice
	firstSeen  time.Time
	lastSeen   time.Time
	label      string
}

// BackfillPersistentTopics reconstructs persistent topics for a board's
// historical sections that have no topic assignment yet. Unlike the daily
// assignment (which opens candidates), backfill creates ACTIVE topics
// directly for clusters of >= 2 sections — these already constitute
// historical evidence of a durable narrative, so there is no observation
// period to serve. Single-member clusters (isolated sections) do NOT seed a
// topic: they stay unassigned and are left for the daily candidate path to
// observe over consecutive days (avoids single-section noise lanes).
//
// Clustering is COMPLETE-LINK agglomerative: sections are processed in date
// order, and a section joins an existing cluster only when it is within
// ClusterThreshold of EVERY member of that cluster; otherwise it seeds a new
// cluster. Complete-link was chosen over running-mean centroid clustering
// after real-data diagnosis showed centroid clustering chains (board 1980: 47
// sections → 1 topic at thr=0.30). The threshold is tuned by
// LoadPersistentTopicConfig (default 0.25) and validated against real data —
// see the change verification-report.
//
// After assignment, RebuildBoardRelations must be called (by the caller or in
// the same transaction) so identity edges pick up the new topic ids.
func (r *TopicGraphRepository) BackfillPersistentTopics(boardID uint) (topicsCreated int, err error) {
	return r.backfillTopics(boardID, true)
}

// BackfillAllPersistentTopics runs the backfill for every board that has
// unassigned sections. Returns per-board counts.
func (r *TopicGraphRepository) BackfillAllPersistentTopics() (map[uint]int, error) {
	var boardIDs []uint
	if err := r.db.Raw(`
		SELECT DISTINCT r.semantic_board_id
		FROM daily_report_sections s
		JOIN board_daily_reports r ON r.id = s.report_id
		WHERE s.persistent_topic_id IS NULL
	`).Scan(&boardIDs).Error; err != nil {
		return nil, fmt.Errorf("list boards with unassigned sections: %w", err)
	}
	result := make(map[uint]int, len(boardIDs))
	for _, bid := range boardIDs {
		n, err := r.backfillTopics(bid, true)
		if err != nil {
			logging.Warnf("BackfillAllPersistentTopics: board %d failed: %v", bid, err)
			continue
		}
		result[bid] = n
	}
	return result, nil
}

// backfillTopics does the clustering + topic creation + section backfill for
// one board. When rebuildRelations is true it also rebuilds relations so
// identity edges pick up the new assignments.
func (r *TopicGraphRepository) backfillTopics(boardID uint, rebuildRelations bool) (int, error) {
	cfg := LoadPersistentTopicConfig(r.db)

	var created int
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Load unassigned sections in date order, carrying the report date on
		// CreatedAt (ListSectionsByBoardOrdered reuses it as the date carrier).
		all, err := r.ListSectionsByBoardOrdered(boardID)
		if err != nil {
			return err
		}
		// Keep only sections still needing assignment.
		var sections []DailyReportSection
		for _, s := range all {
			if s.PersistentTopicID == nil {
				sections = append(sections, s)
			}
		}
		if len(sections) == 0 {
			return nil
		}

		// Parse embeddings once; sections without embeddings are skipped
		// (they cannot be clustered and remain unassigned).
		type secVec struct {
			idx int
			vec []float64
		}
		var usable []secVec
		for i := range sections {
			v, err := repoParsePgVector(sections[i].Embedding)
			if err != nil || len(v) == 0 {
				continue
			}
			usable = append(usable, secVec{idx: i, vec: v})
		}
		if len(usable) == 0 {
			return nil
		}

		// Complete-link agglomerative clustering: a section joins the first
		// cluster whose EVERY member is within ClusterThreshold; else seeds a
		// new cluster. This bounds each cluster's diameter, preventing the
		// centroid-drift chaining that single-link/centroid methods exhibit on
		// real data (see head comment).
		var clusters []topicCluster
		for _, sv := range usable {
			day := NormalizeReportDate(sections[sv.idx].CreatedAt)
			joined := -1
			for ci := range clusters {
				within := true
				for _, mv := range clusters[ci].memberVecs {
					if cosineDistance(sv.vec, mv) > cfg.ClusterThreshold {
						within = false
						break
					}
				}
				if within {
					joined = ci
					break
				}
			}
			if joined < 0 {
				// seed new cluster
				centroid := make([]float64, len(sv.vec))
				copy(centroid, sv.vec)
				clusters = append(clusters, topicCluster{
					centroid:   centroid,
					memberVecs: [][]float64{append([]float64(nil), sv.vec...)},
					members:    []int{sv.idx},
					firstSeen:  day,
					lastSeen:   day,
					label:      sections[sv.idx].ClusterLabel,
				})
				continue
			}
			// join cluster, update running-mean centroid (for the topic's
			// representative embedding) and append the member vector.
			c := &clusters[joined]
			for j := range sv.vec {
				c.centroid[j] = (c.centroid[j]*float64(len(c.members)) + sv.vec[j]) / float64(len(c.members)+1)
			}
			c.memberVecs = append(c.memberVecs, append([]float64(nil), sv.vec...))
			c.members = append(c.members, sv.idx)
			if day.Before(c.firstSeen) {
				c.firstSeen = day
			}
			if day.After(c.lastSeen) {
				c.lastSeen = day
			}
		}

		// Create one ACTIVE topic per cluster and backfill its members.
		// Min-size gate: a single-member cluster is one isolated section that
		// clusters with nothing else — not durable-narrative evidence on its
		// own. Seeding an active topic from it would bypass the consecutive-
		// hits observation window and produce noise lanes, so such sections
		// stay unassigned and are left for the daily candidate path.
		for ci := range clusters {
			c := &clusters[ci]
			if len(c.members) < 2 {
				continue
			}
			topic := BoardPersistentTopic{
				SemanticBoardID: boardID,
				Label:           c.label,
				Embedding:       sections[c.members[0]].Embedding,
				Status:          TopicStatusActive,
				FirstSeenDate:   c.firstSeen,
				LastSeenDate:    c.lastSeen,
				HitCount:        len(c.members),
				ConsecutiveHits: len(c.members),
			}
			if err := r.CreateTopic(tx, &topic); err != nil {
				return fmt.Errorf("create backfill topic: %w", err)
			}
			created++
			for _, mi := range c.members {
				if err := r.UpdateSectionTopicAssignment(tx, sections[mi].ID, &topic.ID, 0, TopicConfAnchorHit, nil); err != nil {
					return fmt.Errorf("backfill section %d: %w", sections[mi].ID, err)
				}
			}
		}

		if rebuildRelations {
			if err := RebuildBoardRelations(tx, boardID); err != nil {
				logging.Warnf("backfillTopics: relation rebuild failed for board %d: %v", boardID, err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	logging.Infof("persistent-topic: backfilled board %d, created %d topics", boardID, created)
	return created, nil
}
