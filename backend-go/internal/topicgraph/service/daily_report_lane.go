package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/jsonutil"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/platform/tracing"
	"syntopica-backend/internal/topicgraph/repository"
)

// Lane tier values persisted on DailyReportSection.LaneTier
// (l1_direct / l2_llm / l3_new). Empty string leaves the column NULL.
const (
	laneL1Direct = "l1_direct"
	laneL2LLM    = "l2_llm"
	laneL3New    = "l3_new"
)

// LaneCandidate is one candidate topic for an L2 tag together with its cosine
// distance to the tag. Field order is TopicID, Distance so the unit tests can
// positionally initialize it as LaneCandidate{7, 0.2}.
type LaneCandidate struct {
	TopicID  uint
	Distance float64
}

// LaneTagAssign pairs a tag with the topic it nearest-matches and, for L2
// tags, the candidate set the LLM adjudicates over.
type LaneTagAssign struct {
	Tag        repository.TagInput
	TopicID    uint
	Distance   float64
	Candidates []LaneCandidate
}

// LaneBuckets holds the three lanes produced by BucketTagsByCentroid.
type LaneBuckets struct {
	L1 []LaneTagAssign
	L2 []LaneTagAssign
	L3 []repository.TagInput
}

// l2Decision is the parsed LLM adjudication for one L2 tag: keep / switch / new,
// the resolved target topic id, and an off-shortlist flag set when an LLM
// "switch" targeted a topic outside the candidate set (degraded to new).
type l2Decision struct {
	tagID         uint
	decision      string // keep / switch / new
	targetTopicID uint
	offShortlist  bool
}

// clusterChatFn is the swappable LLM hook for the lane pipeline. Tests replace
// it with a fake; the default implementation calls airouter exactly as the
// legacy ClusterTags entry did (temperature 0.1, maxTokens 8192, JSONMode with
// schema, CapabilityDigestPolish). The operation string distinguishes the
// daily_report.decide_l2_tags and daily_report.cluster_new_narrative calls.
var clusterChatFn = func(ctx context.Context, system, user string, schema *airouter.JSONSchema, operation string) (string, error) {
	temperature := 0.1
	maxTokens := 8192
	result, err := airouter.NewRouter().Chat(ctx, airouter.ChatRequest{
		Operation:  operation,
		SessionID:  SessionIDFromContext(ctx),
		Capability: airouter.CapabilityDigestPolish,
		Messages: []airouter.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
		JSONMode:    true,
		JSONSchema:  schema,
		Metadata:    map[string]any{"operation": operation},
	})
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// cosineFloats is the float-slice cosine distance (1 - cosine similarity).
// Mismatched, empty, or zero-norm vectors return MaxFloat64 so the caller
// treats them as non-matches. Distinct from merge.go's cosineDistance, which
// operates on pgvector strings.
func cosineFloats(a, b []float64) float64 {
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

// topicAnchorVec returns the topic's matching anchor: the Centroid (mean of
// recent sections) when present, degrading to the 首义 Embedding otherwise.
// nil when neither parses — the topic cannot anchor any tag.
func topicAnchorVec(t repository.BoardPersistentTopic) []float64 {
	if v, err := parsePgVector(t.Centroid); err == nil && len(v) > 0 {
		return v
	}
	if v, err := parsePgVector(t.Embedding); err == nil && len(v) > 0 {
		return v
	}
	return nil
}

// BucketTagsByCentroid routes each tag to its lane by cosine distance to the
// nearest topic anchor (centroid, else 首义 embedding). Pure — no DB, no LLM.
//
//	dist < L1 && !vacuum              → L1 (direct attach)
//	dist < L1 && vacuum               → L2 (vacuum downgrade)
//	L1 <= dist <= L2                  → L2 (carries top-K candidates)
//	dist > L2                         → L3
//	missing embedding / no anchors    → L3
func BucketTagsByCentroid(tags []repository.TagInput, topics []repository.BoardPersistentTopic, tagEmbeddings map[uint][]float64, cfg repository.PersistentTopicConfig) LaneBuckets {
	var buckets LaneBuckets

	type anchor struct {
		topic *repository.BoardPersistentTopic
		vec   []float64
	}
	anchors := make([]anchor, 0, len(topics))
	for i := range topics {
		vec := topicAnchorVec(topics[i])
		if len(vec) == 0 {
			continue
		}
		anchors = append(anchors, anchor{&topics[i], vec})
	}

	k := cfg.L2CandidateK
	if k <= 0 {
		k = len(anchors)
	}

	for _, tag := range tags {
		emb := tagEmbeddings[tag.ID]
		if len(emb) == 0 || len(anchors) == 0 {
			buckets.L3 = append(buckets.L3, tag)
			continue
		}

		type cand struct {
			id   uint
			dist float64
		}
		cands := make([]cand, 0, len(anchors))
		for _, a := range anchors {
			cands = append(cands, cand{a.topic.ID, cosineFloats(emb, a.vec)})
		}
		sort.Slice(cands, func(i, j int) bool { return cands[i].dist < cands[j].dist })

		nearestID := cands[0].id
		nearestDist := cands[0].dist
		var nearestTopic *repository.BoardPersistentTopic
		for _, a := range anchors {
			if a.topic.ID == nearestID {
				nearestTopic = a.topic
				break
			}
		}

		topK := k
		if topK > len(cands) {
			topK = len(cands)
		}
		candidates := make([]LaneCandidate, 0, topK)
		for i := 0; i < topK; i++ {
			candidates = append(candidates, LaneCandidate{TopicID: cands[i].id, Distance: cands[i].dist})
		}

		switch {
		case nearestDist < cfg.LaneL1Threshold && nearestTopic != nil && !nearestTopic.IsVacuum:
			buckets.L1 = append(buckets.L1, LaneTagAssign{Tag: tag, TopicID: nearestID, Distance: nearestDist})
		case nearestDist <= cfg.LaneL2Threshold:
			// (dist < L1 && vacuum) OR (L1 <= dist <= L2) → L2.
			buckets.L2 = append(buckets.L2, LaneTagAssign{Tag: tag, TopicID: nearestID, Distance: nearestDist, Candidates: candidates})
		default:
			buckets.L3 = append(buckets.L3, tag)
		}
	}
	return buckets
}

// parseL2Response maps the LLM's L2 adjudication JSON onto a per-l2-tag
// decision slice, parallel and equal-length to the input l2. Unknown / invalid
// JSON or an omitted tag defaults to keep (anchor nearest). An LLM "switch" to
// a topic outside the candidate set degrades to new and sets offShortlist.
func parseL2Response(content string, l2 []LaneTagAssign) []l2Decision {
	type rawDecision struct {
		TagID         uint   `json:"tag_id"`
		Decision      string `json:"decision"`
		TargetTopicID *uint  `json:"target_topic_id"`
	}
	var raw struct {
		Decisions []rawDecision `json:"decisions"`
	}
	byTag := make(map[uint]rawDecision, len(l2))
	if cleaned := jsonutil.SanitizeLLMJSON(content); cleaned != "" {
		if err := json.Unmarshal([]byte(cleaned), &raw); err == nil {
			for _, d := range raw.Decisions {
				byTag[d.TagID] = d
			}
		}
	}

	out := make([]l2Decision, len(l2))
	for i, a := range l2 {
		var nearest uint
		if len(a.Candidates) > 0 {
			nearest = a.Candidates[0].TopicID
		}
		dec := l2Decision{tagID: a.Tag.ID, decision: "keep", targetTopicID: nearest}
		if rd, ok := byTag[a.Tag.ID]; ok {
			switch rd.Decision {
			case "keep":
				dec.decision = "keep"
				dec.targetTopicID = nearest
			case "switch":
				if rd.TargetTopicID != nil && inCandidateSet(*rd.TargetTopicID, a.Candidates) {
					dec.decision = "switch"
					dec.targetTopicID = *rd.TargetTopicID
				} else {
					dec.decision = "new"
					dec.targetTopicID = 0
					dec.offShortlist = true
				}
			case "new":
				dec.decision = "new"
				dec.targetTopicID = 0
			default:
				dec.decision = "keep"
				dec.targetTopicID = nearest
			}
		}
		out[i] = dec
	}
	return out
}

func inCandidateSet(topicID uint, cands []LaneCandidate) bool {
	for _, c := range cands {
		if c.TopicID == topicID {
			return true
		}
	}
	return false
}

// assembleLaneGroups aggregates L1 + L2-keep/switch tags by their owning topic
// (L1 tags first, then L2 keep/switch), sets Lane="l1" on any topic group that
// has at least one L1 tag (else "l2"), and appends the L3 new-narrative groups
// verbatim. L2 "new" decisions are skipped here — they were routed to the L3
// pool upstream. Topic groups are emitted in ascending topicID order.
func assembleLaneGroups(l1 []LaneTagAssign, l2dec []l2Decision, l3 []repository.ClusterGroup) []repository.ClusterGroup {
	type acc struct {
		tagIDs []uint
		hasL1  bool
	}
	groups := make(map[uint]*acc)
	ensure := func(id uint) *acc {
		if groups[id] == nil {
			groups[id] = &acc{}
		}
		return groups[id]
	}

	for _, a := range l1 {
		g := ensure(a.TopicID)
		g.tagIDs = append(g.tagIDs, a.Tag.ID)
		g.hasL1 = true
	}
	for _, d := range l2dec {
		if d.decision != "keep" && d.decision != "switch" {
			continue
		}
		g := ensure(d.targetTopicID)
		g.tagIDs = append(g.tagIDs, d.tagID)
	}

	topicIDs := make([]uint, 0, len(groups))
	for id := range groups {
		topicIDs = append(topicIDs, id)
	}
	sort.Slice(topicIDs, func(i, j int) bool { return topicIDs[i] < topicIDs[j] })

	result := make([]repository.ClusterGroup, 0, len(topicIDs)+len(l3))
	for _, id := range topicIDs {
		tid := id
		lane := "l2"
		if groups[id].hasL1 {
			lane = "l1"
		}
		result = append(result, repository.ClusterGroup{
			GroupName:      "",
			TagIDs:         groups[id].tagIDs,
			MatchedTopicID: &tid,
			Lane:           lane,
		})
	}
	result = append(result, l3...)
	return result
}

// clusterLaneToTier maps a lane tag onto the persisted LaneTier column value.
// Unknown lanes (incl. "") map to "" so the column stays NULL.
func clusterLaneToTier(lane string) string {
	switch lane {
	case "l1":
		return laneL1Direct
	case "l2":
		return laneL2LLM
	case "l3":
		return laneL3New
	default:
		return ""
	}
}

// ClusterTagsLane is the lane-driven clustering entry point for the daily
// report orchestrator. It buckets tags by centroid distance (L1/L2/L3),
// adjudicates the L2 band with an LLM, groups the L3 band as new narrative,
// and emits []ClusterGroup carrying Lane + MatchedTopicID for the orchestrator
// to stamp onto each section.
func ClusterTagsLane(ctx context.Context, tags []repository.TagInput, topics []repository.BoardPersistentTopic, tagEmbeddings map[uint][]float64, briefs map[uint][]repository.TopicRecentBrief, cfg repository.PersistentTopicConfig) ([]repository.ClusterGroup, error) {
	ctx, span := tracing.Tracer(tracing.ServiceName).Start(ctx, "workflow.daily_report.cluster_tags_lane")
	defer span.End()

	if len(tags) == 0 {
		return nil, nil
	}

	topicByID := make(map[uint]repository.BoardPersistentTopic, len(topics))
	for _, t := range topics {
		topicByID[t.ID] = t
	}

	buckets := BucketTagsByCentroid(tags, topics, tagEmbeddings, cfg)
	l1 := buckets.L1
	l2 := buckets.L2
	l3pool := buckets.L3

	// Fallback: too few ambiguous tags to justify any LLM call. L1 still
	// aggregates per-topic; each L2 tag anchors to its nearest candidate
	// (keep); each L3 tag forms its own group. Zero LLM calls.
	if len(l2)+len(l3pool) <= 2 {
		l2dec := make([]l2Decision, len(l2))
		for i, a := range l2 {
			l2dec[i] = l2Decision{tagID: a.Tag.ID, decision: "keep", targetTopicID: a.TopicID}
		}
		l3groups := make([]repository.ClusterGroup, 0, len(l3pool))
		for _, t := range l3pool {
			l3groups = append(l3groups, repository.ClusterGroup{GroupName: t.Label, TagIDs: []uint{t.ID}, Lane: "l3"})
		}
		groups := assembleLaneGroups(l1, l2dec, l3groups)
		logging.Infof("daily-report: lane cluster (fallback, no LLM) %d tags → l1=%d l2=%d l3=%d groups=%d",
			len(tags), len(l1), len(l2), len(l3pool), len(groups))
		return groups, nil
	}

	// Normal path: L2 LLM adjudication over each tag's candidate set.
	system, user := buildL2Prompt(l2, topicByID, briefs)
	content, err := clusterChatFn(ctx, system, user, l2DecisionSchema(), "daily_report.decide_l2_tags")
	if err != nil {
		return nil, fmt.Errorf("l2 decide AI call failed: %w", err)
	}
	l2dec := parseL2Response(content, l2)

	// Route L2 "new" decisions (incl. off-shortlist downgrades) into the L3 pool.
	for i, d := range l2dec {
		if d.decision == "new" && i < len(l2) {
			l3pool = append(l3pool, l2[i].Tag)
		}
	}

	// L3 new-narrative grouping.
	var l3groups []repository.ClusterGroup
	if len(l3pool) > 2 {
		// Pure new-narrative grouping: no existing-topic framework injection
		// (pass nil topics so matched_topic_id stays null). Reuses the legacy
		// title rules from buildClusterSystemPrompt/buildClusterPrompt.
		sys := buildClusterSystemPrompt(len(l3pool), nil, nil)
		usr := buildClusterPrompt(l3pool)
		l3content, l3err := clusterChatFn(ctx, sys, usr, l3GroupsSchema(), "daily_report.cluster_new_narrative")
		if l3err != nil {
			return nil, fmt.Errorf("l3 cluster AI call failed: %w", l3err)
		}
		parsed, perr := parseClusterResponse(l3content, l3pool, nil)
		if perr != nil {
			return nil, fmt.Errorf("parse l3 cluster response: %w", perr)
		}
		l3groups = parsed
		for i := range l3groups {
			l3groups[i].Lane = "l3"
			l3groups[i].MatchedTopicID = nil
		}
	} else {
		l3groups = make([]repository.ClusterGroup, 0, len(l3pool))
		for _, t := range l3pool {
			l3groups = append(l3groups, repository.ClusterGroup{GroupName: t.Label, TagIDs: []uint{t.ID}, Lane: "l3"})
		}
	}

	groups := assembleLaneGroups(l1, l2dec, l3groups)
	logging.Infof("daily-report: lane cluster %d tags → l1=%d l2=%d l3=%d groups=%d",
		len(tags), len(l1), len(l2), len(l3pool), len(groups))
	return groups, nil
}

// buildL2Prompt constructs the L2 adjudication system + user prompts. Each L2
// tag is rendered with its top-K candidate topics (label, status, distance) and
// each candidate's recent section/thread content (when briefs are available) so
// the LLM judges attribution by narrative substance, not label surface.
func buildL2Prompt(l2 []LaneTagAssign, topicByID map[uint]repository.BoardPersistentTopic, briefs map[uint][]repository.TopicRecentBrief) (system, user string) {
	system = `你是一名专业的新闻叙事分析师。下面给出若干"待定"标签，每个标签附带其最近的若干已有话题候选（含近期内容）。请判断每个标签的归属：
- keep：标签应归入其最近候选话题（语义最贴近的现有框架），target_topic_id 填该最近候选
- switch：标签应归入候选集内的另一个话题（而非最近候选），target_topic_id 填该话题
- new：标签属于尚未覆盖的全新叙事，不归属任何候选，target_topic_id 省略或为 0

判断须基于候选话题的实际近期内容，而非仅凭标题字面沾边；两个话题即使标题字面相似，若各自近期内容分属不同叙事，应视为不同框架。
只返回合法 JSON，不要 Markdown 代码块或解释文字：{"decisions":[{"tag_id":整数,"decision":"keep|switch|new","target_topic_id":整数}]}`

	var sb strings.Builder
	sb.WriteString("## 待定标签与候选话题\n\n")
	for _, a := range l2 {
		fmt.Fprintf(&sb, "### 标签 [ID:%d] %s\n", a.Tag.ID, a.Tag.Label)
		if a.Tag.Description != "" {
			fmt.Fprintf(&sb, "描述: %s\n", a.Tag.Description)
		}
		if a.Tag.ArticleContext != "" {
			fmt.Fprintf(&sb, "代表文章: %s\n", a.Tag.ArticleContext)
		}
		sb.WriteString("候选话题:\n")
		for _, c := range a.Candidates {
			t, ok := topicByID[c.TopicID]
			if !ok {
				continue
			}
			statusLabel := "正式"
			if t.Status == repository.TopicStatusCandidate {
				statusLabel = "观察中"
			}
			fmt.Fprintf(&sb, "- [topic:%d] %s（状态:%s，最近命中:%s，累计%d天，距离%.3f）\n",
				t.ID, t.Label, statusLabel, t.LastSeenDate.Format("2006-01-02"), t.HitCount, c.Distance)
			if items := briefs[t.ID]; len(items) > 0 {
				sb.WriteString("  近期内容:")
				for _, item := range items {
					fmt.Fprintf(&sb, "\n  - section \"%s\" (%s)", item.SectionLabel, item.PeriodDate.Format("2006-01-02"))
					if len(item.ThreadTitles) > 0 {
						sb.WriteString(": ")
						for j, tt := range item.ThreadTitles {
							if j > 0 {
								sb.WriteString(", ")
							}
							fmt.Fprintf(&sb, "thread \"%s\"", tt)
						}
					}
				}
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("请为以上每个标签给出 keep / switch / new 决策。\n")
	return system, sb.String()
}

// l2DecisionSchema is the JSON schema for the L2 adjudication response.
func l2DecisionSchema() *airouter.JSONSchema {
	return &airouter.JSONSchema{
		Type: "object",
		Properties: map[string]airouter.SchemaProperty{
			"decisions": {
				Type: "array",
				Items: &airouter.SchemaProperty{
					Type: "object",
					Properties: map[string]airouter.SchemaProperty{
						"tag_id":          {Type: "integer"},
						"decision":        {Type: "string", Enum: []string{"keep", "switch", "new"}},
						"target_topic_id": {Type: "integer"},
					},
					Required: []string{"tag_id", "decision"},
				},
			},
		},
		Required: []string{"decisions"},
	}
}

// l3GroupsSchema is the JSON schema for the L3 new-narrative grouping response.
// No matched_topic_id: L3 groups are brand-new narratives.
func l3GroupsSchema() *airouter.JSONSchema {
	return &airouter.JSONSchema{
		Type: "object",
		Properties: map[string]airouter.SchemaProperty{
			"groups": {
				Type: "array",
				Items: &airouter.SchemaProperty{
					Type: "object",
					Properties: map[string]airouter.SchemaProperty{
						"group_name": {Type: "string", Description: "组名，不超过20字"},
						"tag_ids":    {Type: "array", Items: &airouter.SchemaProperty{Type: "integer"}},
					},
					Required: []string{"group_name", "tag_ids"},
				},
			},
		},
		Required: []string{"groups"},
	}
}
