package repository

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"syntopica-backend/internal/platform/logging"

	"gorm.io/gorm"
)

// repoParsePgVector parses a pgvector string "[0.1,0.2]" into a float slice.
// Mirrors the service-layer parsePgVector; kept local to avoid an import cycle
// (service imports repository, not the reverse).
func repoParsePgVector(s string) ([]float64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil, fmt.Errorf("empty vector")
	}
	parts := strings.Split(s, ",")
	result := make([]float64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("parse vector element %d: %w", i, err)
		}
		result[i] = v
	}
	return result, nil
}

// cosineDistance = 1 - cosineSimilarity. Mismatched or zero-length vectors
// return +Inf so the caller treats them as non-matches.
func cosineDistance(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return math.MaxFloat64
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return math.MaxFloat64
	}
	return 1 - dot/(math.Sqrt(na)*math.Sqrt(nb))
}

// parsedTopic pairs a topic id with its pre-parsed embedding.
type parsedTopic struct {
	ID        uint
	Embedding []float64
}

// parseTopicEmbeddings parses each topic's embedding once, dropping unparseable
// rows (they cannot participate in nearest-neighbour assignment).
func parseTopicEmbeddings(topics []BoardPersistentTopic) []parsedTopic {
	out := make([]parsedTopic, 0, len(topics))
	for _, t := range topics {
		vec, err := repoParsePgVector(t.Embedding)
		if err != nil || len(vec) == 0 {
			continue
		}
		out = append(out, parsedTopic{ID: t.ID, Embedding: vec})
	}
	return out
}

// findNearestTopic returns the topic id with the smallest cosine distance to
// the given embedding and that distance. ok=false when no comparable topic.
func findNearestTopic(vec []float64, topics []parsedTopic) (id uint, dist float64, ok bool) {
	best := math.MaxFloat64
	var bestID uint
	found := false
	for _, t := range topics {
		if len(t.Embedding) == 0 {
			continue
		}
		d := cosineDistance(vec, t.Embedding)
		if d < best {
			best = d
			bestID = t.ID
			found = true
		}
	}
	return bestID, best, found
}

// nearbyTopic pairs a parsed topic with its distance to a query vector.
type nearbyTopic struct {
	parsedTopic
	dist float64
}

// findTopicsWithinThreshold returns every topic whose cosine embedding
// distance to vec is <= threshold, sorted nearest-first. Empty when none
// qualify (the section is embedding-far from every active topic).
func findTopicsWithinThreshold(vec []float64, topics []parsedTopic, threshold float64) []nearbyTopic {
	var out []nearbyTopic
	for _, t := range topics {
		if len(t.Embedding) == 0 {
			continue
		}
		d := cosineDistance(vec, t.Embedding)
		if d <= threshold {
			out = append(out, nearbyTopic{parsedTopic: t, dist: d})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].dist < out[j].dist })
	return out
}

// findTopicInList returns the topic with the given id from a nearby list.
func findTopicInList(list []nearbyTopic, id uint) (nearbyTopic, bool) {
	for _, t := range list {
		if t.ID == id {
			return t, true
		}
	}
	return nearbyTopic{}, false
}

// topicAssignmentDecision is the per-section outcome of the dual-confirmation
// assignment. topicID is set for anchor_hit; newCandidate is set for auto_new.
type topicAssignmentDecision struct {
	sectionIdx   int
	topicID      uint
	distance     float64
	confidence   string
	newCandidate *candidateTopicSpec
}

// candidateTopicSpec describes a candidate topic to insert during assignment.
type candidateTopicSpec struct {
	label     string
	embedding string
	firstSeen time.Time
}

// planTopicAssignments computes the per-section assignment plan with NO DB
// access — pure and unit-testable. For each section it either anchors to an
// existing topic (dual confirmation: nearest embedding within threshold AND
// the LLM's matched_topic_id agrees) or opens a new candidate (auto_new), or
// is marked unmatched when it has no embedding. Candidate specs are collected
// in the returned slice; each auto_new decision points at its spec.
func planTopicAssignments(sections []DailyReportSection, existingTopics []BoardPersistentTopic, cfg PersistentTopicConfig, today time.Time) []topicAssignmentDecision {
	parsed := parseTopicEmbeddings(existingTopics)
	decisions := make([]topicAssignmentDecision, 0, len(sections))
	var candidates []candidateTopicSpec
	for i := range sections {
		sec := &sections[i]
		vec, err := repoParsePgVector(sec.Embedding)
		if err != nil || len(vec) == 0 {
			decisions = append(decisions, topicAssignmentDecision{
				sectionIdx: i, confidence: TopicConfUnmatched,
			})
			continue
		}
		// Dual confirmation: the LLM's matched_topic_id must point at a topic
		// whose embedding is within the match threshold. Pre-relaxation this
		// required matched_id == single nearest; that was too brittle — any
		// drift in LLM clustering (e.g. the quality-scoring truncation change)
		// made the LLM pick the 2nd-nearest and severed anchoring en masse.
		// Accepting any within-threshold topic keeps BOTH gates (embedding AND
		// the LLM naming the same topic) while tolerating minor drift.
		within := findTopicsWithinThreshold(vec, parsed, cfg.MatchThreshold)
		if sec.MatchedTopicID != nil {
			if pt, ok := findTopicInList(within, *sec.MatchedTopicID); ok {
				decisions = append(decisions, topicAssignmentDecision{
					sectionIdx: i, topicID: pt.ID, distance: pt.dist,
					confidence: TopicConfAnchorHit,
				})
				continue
			}
		}
		_, nearestDist, _ := findNearestTopic(vec, parsed)
		candidates = append(candidates, candidateTopicSpec{
			label:     sec.ClusterLabel,
			embedding: sec.Embedding,
			firstSeen: today,
		})
		decisions = append(decisions, topicAssignmentDecision{
			sectionIdx: i, distance: nearestDist,
			confidence:   TopicConfAutoNew,
			newCandidate: &candidates[len(candidates)-1],
		})
	}
	return decisions
}

// topicLifecycleChange is a pure lifecycle transition computed without DB
// access. Updated fields only; the caller persists them.
type topicLifecycleChange struct {
	topicID         uint
	status          string
	consecutiveHits int
	hitCount        int
	lastSeen        time.Time
}

// planLifecycle computes the candidate→active→archived transitions for a
// board's topics given today's hit set. Pure and unit-testable.
//
// hit: consecutive_hits+1, hit_count+1, last_seen=today. Candidates remain
// candidates until a user confirms them after they reach UpgradeThreshold.
// miss: consecutive_hits reset to 0; active archives after DecayWindow days.
//
// New candidates created by the assignment step already carry consecutive_hits=1
// in the DB; they are NOT passed through here (they are in hitTopicIDs only if
// the caller includes them, which it should not — see assignAndUpdateTopics).
func planLifecycle(topics []BoardPersistentTopic, today time.Time, hitTopicIDs map[uint]bool, cfg PersistentTopicConfig) []topicLifecycleChange {
	todayDate := NormalizeReportDate(today)
	var changes []topicLifecycleChange
	for _, t := range topics {
		lastSeen := NormalizeReportDate(t.LastSeenDate)
		if hitTopicIDs[t.ID] {
			newHits := t.ConsecutiveHits + 1
			ch := topicLifecycleChange{
				topicID: t.ID,
				status:  t.Status, consecutiveHits: newHits,
				hitCount: t.HitCount + 1, lastSeen: todayDate,
			}
			changes = append(changes, ch)
			continue
		}
		if t.Status == TopicStatusCandidate && t.ConsecutiveHits != 0 {
			changes = append(changes, topicLifecycleChange{
				topicID: t.ID, status: TopicStatusCandidate,
				consecutiveHits: 0, hitCount: t.HitCount, lastSeen: lastSeen,
			})
			continue
		}
		if t.Status == TopicStatusActive {
			gapDays := int(todayDate.Sub(lastSeen).Hours() / 24)
			if gapDays > cfg.DecayWindow {
				changes = append(changes, topicLifecycleChange{
					topicID: t.ID, status: TopicStatusArchived,
					consecutiveHits: 0, hitCount: t.HitCount, lastSeen: lastSeen,
				})
			}
		}
	}
	return changes
}

// assignAndUpdateTopics runs the full assignment + lifecycle pipeline for one
// report save, inside the caller's transaction:
//  1. load existing anchorable topics + config
//  2. plan per-section assignment (pure, dual confirmation)
//  3. create candidate topics
//  4. write assignment columns onto sections
//  5. advance the topic lifecycle (candidate→active→archived)
//
// Identity edges written later by RebuildBoardRelations depend on the
// persistent_topic_id set here, so this must run before relation rebuild and
// inside the same transaction.
func assignAndUpdateTopics(tx *gorm.DB, boardID uint, periodDate time.Time, sections []DailyReportSection) error {
	cfg := LoadPersistentTopicConfig(tx)
	existingTopics, err := Repo.ListActiveTopicsByBoard(boardID)
	if err != nil {
		return fmt.Errorf("load existing topics: %w", err)
	}
	today := NormalizeReportDate(periodDate)
	decisions := planTopicAssignments(sections, existingTopics, cfg, today)

	// Create candidates and remember their ids.
	candSpecs := make([]*candidateTopicSpec, 0, len(decisions))
	for i := range decisions {
		if decisions[i].newCandidate != nil {
			candSpecs = append(candSpecs, decisions[i].newCandidate)
		}
	}
	candIDs := make(map[*candidateTopicSpec]uint, len(candSpecs))
	for _, spec := range candSpecs {
		topic := BoardPersistentTopic{
			SemanticBoardID: boardID,
			Label:           spec.label,
			Embedding:       spec.embedding,
			Status:          TopicStatusCandidate,
			FirstSeenDate:   spec.firstSeen,
			LastSeenDate:    spec.firstSeen,
			HitCount:        1,
			ConsecutiveHits: 1,
		}
		if err := Repo.CreateTopic(tx, &topic); err != nil {
			return fmt.Errorf("create candidate topic: %w", err)
		}
		candIDs[spec] = topic.ID
	}

	// Write assignment columns and collect the set of hit topic ids.
	// Newly created candidates are already at consecutive_hits=1, so they are
	// excluded from the lifecycle hit set to avoid double-counting.
	hitTopicIDs := make(map[uint]bool, len(decisions))
	for _, d := range decisions {
		var topicID *uint
		switch d.confidence {
		case TopicConfAnchorHit:
			id := d.topicID
			topicID = &id
			hitTopicIDs[id] = true
		case TopicConfAutoNew:
			if d.newCandidate != nil {
				id := candIDs[d.newCandidate]
				topicID = &id
			}
		}
		if d.sectionIdx >= len(sections) {
			continue
		}
		secID := sections[d.sectionIdx].ID
		if err := Repo.UpdateSectionTopicAssignment(tx, secID, topicID, d.distance, d.confidence); err != nil {
			return fmt.Errorf("update section %d assignment: %w", secID, err)
		}
	}

	// Advance the lifecycle for pre-existing topics only.
	allTopics, err := Repo.ListAllTopicsByBoard(boardID)
	if err != nil {
		return fmt.Errorf("load topics for lifecycle: %w", err)
	}
	changes := planLifecycle(allTopics, today, hitTopicIDs, cfg)
	if len(changes) > 0 {
		toSave := make([]BoardPersistentTopic, 0, len(changes))
		for _, ch := range changes {
			for _, t := range allTopics {
				if t.ID == ch.topicID {
					t.Status = ch.status
					t.ConsecutiveHits = ch.consecutiveHits
					t.HitCount = ch.hitCount
					t.LastSeenDate = ch.lastSeen
					toSave = append(toSave, t)
					break
				}
			}
		}
		if err := Repo.SaveTopics(tx, toSave); err != nil {
			return fmt.Errorf("save topic lifecycle changes: %w", err)
		}
	}

	matched, autoNew, unmatched := 0, 0, 0
	for _, d := range decisions {
		switch d.confidence {
		case TopicConfAnchorHit:
			matched++
		case TopicConfAutoNew:
			autoNew++
		case TopicConfUnmatched:
			unmatched++
		}
	}
	logging.Infof("persistent-topic: board %d assigned %d sections (anchor_hit=%d, auto_new=%d, unmatched=%d)",
		boardID, len(decisions), matched, autoNew, unmatched)
	return nil
}
