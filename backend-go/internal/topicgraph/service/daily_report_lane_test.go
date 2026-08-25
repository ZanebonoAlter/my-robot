package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/topicgraph/repository"
)

// unit returns the unit vector for a Pythagorean (cos, sin) pair so cosine
// distances from [1,0] are exact-ish: cosineDist([1,0], unit(c,s)) = 1-c.
func unit(c, s float64) []float64 { return []float64{c, s} }

func laneCfg() repository.PersistentTopicConfig {
	return repository.PersistentTopicConfig{
		LaneL1Threshold: 0.18,
		LaneL2Threshold: 0.30,
		L2CandidateK:    5,
	}
}

// ---- BucketTagsByCentroid ----

func TestBucketTagsByCentroid_L1L2L3(t *testing.T) {
	// Topic at [1,0]. Tags at clean Pythagorean distances:
	//   [1,0]     → dist 0.0  (< 0.18 → L1)
	//   [0.8,0.6] → dist 0.2  (in [0.18,0.30] → L2)
	//   [0.6,0.8] → dist 0.4  (> 0.30 → L3)
	topic := repository.BoardPersistentTopic{ID: 7, Centroid: repository.FloatsToPgVector(unit(1, 0))}
	tags := []repository.TagInput{
		{ID: 1, Label: "strong"},
		{ID: 2, Label: "weak"},
		{ID: 3, Label: "far"},
	}
	emb := map[uint][]float64{
		1: unit(1, 0),
		2: unit(0.8, 0.6),
		3: unit(0.6, 0.8),
	}
	b := BucketTagsByCentroid(tags, []repository.BoardPersistentTopic{topic}, emb, laneCfg())

	require.Len(t, b.L1, 1)
	assert.Equal(t, uint(1), b.L1[0].Tag.ID)
	assert.Equal(t, uint(7), b.L1[0].TopicID)

	require.Len(t, b.L2, 1)
	assert.Equal(t, uint(2), b.L2[0].Tag.ID)
	assert.Equal(t, uint(7), b.L2[0].TopicID)
	require.Len(t, b.L2[0].Candidates, 1, "L2 carries its candidate topic")

	require.Len(t, b.L3, 1)
	assert.Equal(t, uint(3), b.L3[0].ID)
}

func TestBucketTagsByCentroid_BoundaryL1IsL2(t *testing.T) {
	// dist == L1 (strict <) → L2. Use a threshold just below the tag's distance.
	topic := repository.BoardPersistentTopic{ID: 7, Centroid: repository.FloatsToPgVector(unit(1, 0))}
	tags := []repository.TagInput{{ID: 1, Label: "at-l1"}}
	emb := map[uint][]float64{1: unit(0.8, 0.6)} // dist ~0.2
	cfg := laneCfg()
	cfg.LaneL1Threshold = 0.1999 // tag dist 0.2 is NOT < 0.1999 → L2
	b := BucketTagsByCentroid(tags, []repository.BoardPersistentTopic{topic}, emb, cfg)
	require.Len(t, b.L2, 1)
	assert.Empty(t, b.L1)
}

func TestBucketTagsByCentroid_BoundaryL2IsL2(t *testing.T) {
	// dist <= L2 → L2 (not L3). Tag dist ~0.2, L2 threshold 0.2.
	topic := repository.BoardPersistentTopic{ID: 7, Centroid: repository.FloatsToPgVector(unit(1, 0))}
	tags := []repository.TagInput{{ID: 1, Label: "at-l2"}}
	emb := map[uint][]float64{1: unit(0.8, 0.6)}
	cfg := laneCfg()
	cfg.LaneL1Threshold = 0.10
	cfg.LaneL2Threshold = 0.21 // tag dist 0.2 <= 0.21 → L2
	b := BucketTagsByCentroid(tags, []repository.BoardPersistentTopic{topic}, emb, cfg)
	require.Len(t, b.L2, 1)
	assert.Empty(t, b.L3)
}

func TestBucketTagsByCentroid_BeyondL2IsL3(t *testing.T) {
	topic := repository.BoardPersistentTopic{ID: 7, Centroid: repository.FloatsToPgVector(unit(1, 0))}
	tags := []repository.TagInput{{ID: 1, Label: "far"}}
	emb := map[uint][]float64{1: unit(0.6, 0.8)} // dist ~0.4 > 0.30
	b := BucketTagsByCentroid(tags, []repository.BoardPersistentTopic{topic}, emb, laneCfg())
	require.Len(t, b.L3, 1)
}

func TestBucketTagsByCentroid_VacuumDowngrade(t *testing.T) {
	// Tag strong-matches a vacuum topic (dist < L1) → downgraded to L2.
	topic := repository.BoardPersistentTopic{ID: 7, Centroid: repository.FloatsToPgVector(unit(1, 0)), IsVacuum: true}
	tags := []repository.TagInput{{ID: 1, Label: "would-be-l1"}}
	emb := map[uint][]float64{1: unit(1, 0)} // dist 0.0
	b := BucketTagsByCentroid(tags, []repository.BoardPersistentTopic{topic}, emb, laneCfg())
	require.Len(t, b.L2, 1, "vacuum topic forces strong-match tag to L2")
	assert.Empty(t, b.L1)
}

// ---- candidate L1 gate (candidate-topic-l2-gate) ----

// gateCfg returns a lane config with the candidate gate explicitly on
// (mirrors the production default from DefaultPersistentTopicConfig).
func gateCfg() repository.PersistentTopicConfig {
	cfg := laneCfg()
	cfg.CandidateL1GateEnabled = true
	return cfg
}

// TestBucketTagsByCentroid_CandidateNearDistanceDowngradesL2 covers spec
// scenario「candidate 近距离降级 L2（观察期门禁）」: with the gate on, a
// near-distance tag (dist < L1) whose nearest topic is candidate SHALL land in
// L2 carrying its candidate set, NOT direct-attach.
func TestBucketTagsByCentroid_CandidateNearDistanceDowngradesL2(t *testing.T) {
	topic := repository.BoardPersistentTopic{ID: 7, Centroid: repository.FloatsToPgVector(unit(1, 0)), Status: repository.TopicStatusCandidate}
	tags := []repository.TagInput{{ID: 1, Label: "candidate-near"}}
	emb := map[uint][]float64{1: unit(1, 0)} // dist 0.0 < 0.18
	b := BucketTagsByCentroid(tags, []repository.BoardPersistentTopic{topic}, emb, gateCfg())
	require.Len(t, b.L2, 1, "gate on: candidate near-distance tag downgraded to L2")
	assert.Empty(t, b.L1, "gate on: no direct attach to candidate topic")
	require.Len(t, b.L2[0].Candidates, 1, "L2 assign carries its candidate set")
	assert.Equal(t, uint(7), b.L2[0].Candidates[0].TopicID)
}

// TestBucketTagsByCentroid_GateOffRestoresLegacyL1 covers spec scenario
// 「门禁开关关闭回退旧行为」: with the gate off, a near-distance tag whose
// nearest topic is candidate direct-attaches (legacy behavior).
func TestBucketTagsByCentroid_GateOffRestoresLegacyL1(t *testing.T) {
	topic := repository.BoardPersistentTopic{ID: 7, Centroid: repository.FloatsToPgVector(unit(1, 0)), Status: repository.TopicStatusCandidate}
	tags := []repository.TagInput{{ID: 1, Label: "legacy-l1"}}
	emb := map[uint][]float64{1: unit(1, 0)}
	cfg := laneCfg()
	cfg.CandidateL1GateEnabled = false
	b := BucketTagsByCentroid(tags, []repository.BoardPersistentTopic{topic}, emb, cfg)
	require.Len(t, b.L1, 1, "gate off: candidate topic direct-attaches (legacy)")
	assert.Empty(t, b.L2)
}

// TestBucketTagsByCentroid_ActiveKeepsL1WithGateOn is a regression guard for
// spec scenario「L1 直挂命中」: with the gate on, ACTIVE topics keep direct
// attach — the gate must only affect candidates.
func TestBucketTagsByCentroid_ActiveKeepsL1WithGateOn(t *testing.T) {
	topic := repository.BoardPersistentTopic{ID: 7, Centroid: repository.FloatsToPgVector(unit(1, 0)), Status: repository.TopicStatusActive}
	tags := []repository.TagInput{{ID: 1, Label: "active-near"}}
	emb := map[uint][]float64{1: unit(1, 0)}
	b := BucketTagsByCentroid(tags, []repository.BoardPersistentTopic{topic}, emb, gateCfg())
	require.Len(t, b.L1, 1, "gate on: active topic still direct-attaches")
	assert.Empty(t, b.L2)
}

// TestBucketTagsByCentroid_VacuumActiveStillL2WithGateOn covers spec scenario
// 「vacuum active 不直挂（既有规则保持）」under the gate: vacuum downgrade is
// orthogonal to the candidate gate.
func TestBucketTagsByCentroid_VacuumActiveStillL2WithGateOn(t *testing.T) {
	topic := repository.BoardPersistentTopic{ID: 7, Centroid: repository.FloatsToPgVector(unit(1, 0)), IsVacuum: true, Status: repository.TopicStatusActive}
	tags := []repository.TagInput{{ID: 1, Label: "vacuum-near"}}
	emb := map[uint][]float64{1: unit(1, 0)}
	b := BucketTagsByCentroid(tags, []repository.BoardPersistentTopic{topic}, emb, gateCfg())
	require.Len(t, b.L2, 1, "vacuum active still downgrades strong-match tag to L2")
	assert.Empty(t, b.L1)
}

func TestBucketTagsByCentroid_NoTopicsAllL3(t *testing.T) {
	tags := []repository.TagInput{{ID: 1, Label: "x"}, {ID: 2, Label: "y"}}
	emb := map[uint][]float64{1: unit(1, 0), 2: unit(0, 1)}
	b := BucketTagsByCentroid(tags, nil, emb, laneCfg())
	require.Len(t, b.L3, 2)
}

func TestBucketTagsByCentroid_MissingEmbeddingL3(t *testing.T) {
	topic := repository.BoardPersistentTopic{ID: 7, Centroid: repository.FloatsToPgVector(unit(1, 0))}
	tags := []repository.TagInput{{ID: 1, Label: "no-emb"}}
	b := BucketTagsByCentroid(tags, []repository.BoardPersistentTopic{topic}, map[uint][]float64{}, laneCfg())
	require.Len(t, b.L3, 1)
}

func TestBucketTagsByCentroid_TopKTruncation(t *testing.T) {
	// Tag at [1,0]; two topics both within L2 (dist 0.2 and 0.25); top-K=1
	// keeps only the nearest (topic 7).
	topics := []repository.BoardPersistentTopic{
		{ID: 7, Centroid: repository.FloatsToPgVector(unit(0.8, 0.6))},      // dist 0.2 (nearest)
		{ID: 8, Centroid: repository.FloatsToPgVector(unit(0.75, 0.66144))}, // dist 0.25
	}
	tags := []repository.TagInput{{ID: 1, Label: "weak"}}
	emb := map[uint][]float64{1: unit(1, 0)}
	cfg := laneCfg()
	cfg.L2CandidateK = 1
	b := BucketTagsByCentroid(tags, topics, emb, cfg)
	require.Len(t, b.L2, 1)
	require.Len(t, b.L2[0].Candidates, 1, "top-K=1 truncates candidates")
	assert.Equal(t, uint(7), b.L2[0].Candidates[0].TopicID, "nearest candidate kept")
}

// ---- parseL2Response ----

func l2Assign(tagID uint, cands ...LaneCandidate) LaneTagAssign {
	return LaneTagAssign{
		Tag: repository.TagInput{ID: tagID},
		TopicID: func() uint {
			if len(cands) > 0 {
				return cands[0].TopicID
			}
			return 0
		}(),
		Candidates: cands,
	}
}

func TestParseL2Response_KeepUsesNearest(t *testing.T) {
	l2 := []LaneTagAssign{l2Assign(1, LaneCandidate{7, 0.2}, LaneCandidate{8, 0.25})}
	content := `{"decisions":[{"tag_id":1,"decision":"keep"}]}`
	dec := parseL2Response(content, l2)
	require.Len(t, dec, 1)
	assert.Equal(t, "keep", dec[0].decision)
	assert.Equal(t, uint(7), dec[0].targetTopicID)
	assert.False(t, dec[0].offShortlist)
}

func TestParseL2Response_KeepHonorsInShortlistTarget(t *testing.T) {
	// 小模型常态混用 keep/switch（实测 ~10-20% 的 keep 会填非最近 target）：
	// keep 却显式指定候选集内另一话题 = LLM 的语义异议。parser SHALL 尊重该
	// 指定，而非静默改写回最近候选——否则 keep 会把 tag 无条件吸附回
	// embedding 最近处（含僵尸 candidate），LLM 判断被丢弃（卡里巴夫案例根因）。
	l2 := []LaneTagAssign{l2Assign(1, LaneCandidate{7, 0.2}, LaneCandidate{8, 0.25})}
	content := `{"decisions":[{"tag_id":1,"decision":"keep","target_topic_id":8}]}`
	dec := parseL2Response(content, l2)
	require.Len(t, dec, 1)
	assert.Equal(t, "keep", dec[0].decision)
	assert.Equal(t, uint(8), dec[0].targetTopicID, "in-shortlist keep target honored")
	assert.False(t, dec[0].offShortlist)
}

func TestParseL2Response_KeepOffShortlistTargetFallsBackNearest(t *testing.T) {
	// keep + 集外/空 target：维持安全网回最近候选（keep 不携带换轨强意图，
	// 不像 off-shortlist switch 那样降级 new）。
	l2 := []LaneTagAssign{l2Assign(1, LaneCandidate{7, 0.2}, LaneCandidate{8, 0.25})}
	content := `{"decisions":[{"tag_id":1,"decision":"keep","target_topic_id":999}]}`
	dec := parseL2Response(content, l2)
	require.Len(t, dec, 1)
	assert.Equal(t, "keep", dec[0].decision)
	assert.Equal(t, uint(7), dec[0].targetTopicID, "off-shortlist keep target falls back to nearest")
	assert.False(t, dec[0].offShortlist)
}

func TestParseL2Response_SwitchInShortlist(t *testing.T) {
	l2 := []LaneTagAssign{l2Assign(1, LaneCandidate{7, 0.2}, LaneCandidate{8, 0.25})}
	content := `{"decisions":[{"tag_id":1,"decision":"switch","target_topic_id":8}]}`
	dec := parseL2Response(content, l2)
	require.Len(t, dec, 1)
	assert.Equal(t, "switch", dec[0].decision)
	assert.Equal(t, uint(8), dec[0].targetTopicID)
}

func TestParseL2Response_SwitchOffShortlistDowngradesNew(t *testing.T) {
	// LLM picks a topic NOT in the candidate set → downgrade to new + flag.
	l2 := []LaneTagAssign{l2Assign(1, LaneCandidate{7, 0.2})}
	content := `{"decisions":[{"tag_id":1,"decision":"switch","target_topic_id":999}]}`
	dec := parseL2Response(content, l2)
	require.Len(t, dec, 1)
	assert.Equal(t, "new", dec[0].decision, "off-shortlist switch degrades to new")
	assert.True(t, dec[0].offShortlist, "off-shortlist flag set")
	assert.Equal(t, uint(0), dec[0].targetTopicID)
}

func TestParseL2Response_NewRouteToL3(t *testing.T) {
	l2 := []LaneTagAssign{l2Assign(1, LaneCandidate{7, 0.2})}
	content := `{"decisions":[{"tag_id":1,"decision":"new"}]}`
	dec := parseL2Response(content, l2)
	require.Len(t, dec, 1)
	assert.Equal(t, "new", dec[0].decision)
}

func TestParseL2Response_MissingTagDefaultsKeep(t *testing.T) {
	// LLM omits a tag → default keep (anchor nearest).
	l2 := []LaneTagAssign{l2Assign(1, LaneCandidate{7, 0.2})}
	dec := parseL2Response(`{"decisions":[]}`, l2)
	require.Len(t, dec, 1)
	assert.Equal(t, "keep", dec[0].decision)
	assert.Equal(t, uint(7), dec[0].targetTopicID)
}

func TestParseL2Response_InvalidJSONAllKeep(t *testing.T) {
	l2 := []LaneTagAssign{
		l2Assign(1, LaneCandidate{7, 0.2}),
		l2Assign(2, LaneCandidate{8, 0.22}),
	}
	dec := parseL2Response(`not json`, l2)
	require.Len(t, dec, 2)
	for _, d := range dec {
		assert.Equal(t, "keep", d.decision)
	}
}

// ---- assembleLaneGroups ----

func TestAssembleLaneGroups_L1AndL2MergePerTopic(t *testing.T) {
	// L1 tag on topic 7 + L2 keep on topic 7 → one group, lane l1 (any L1 tag
	// makes the section l1_direct).
	l1 := []LaneTagAssign{{Tag: repository.TagInput{ID: 1}, TopicID: 7}}
	l2dec := []l2Decision{{tagID: 2, decision: "keep", targetTopicID: 7}}
	l3 := []repository.ClusterGroup{{GroupName: "new", TagIDs: []uint{3}, Lane: "l3"}}

	groups := assembleLaneGroups(l1, l2dec, l3)
	require.Len(t, groups, 2)
	// First group: topic 7, lane l1, both tags.
	require.NotNil(t, groups[0].MatchedTopicID)
	assert.Equal(t, uint(7), *groups[0].MatchedTopicID)
	assert.Equal(t, "l1", groups[0].Lane)
	assert.ElementsMatch(t, []uint{1, 2}, groups[0].TagIDs)
	// L3 group appended.
	assert.Equal(t, "l3", groups[1].Lane)
}

func TestAssembleLaneGroups_L2OnlyIsL2(t *testing.T) {
	l2dec := []l2Decision{{tagID: 1, decision: "keep", targetTopicID: 7}}
	groups := assembleLaneGroups(nil, l2dec, nil)
	require.Len(t, groups, 1)
	assert.Equal(t, "l2", groups[0].Lane)
	require.NotNil(t, groups[0].MatchedTopicID)
	assert.Equal(t, uint(7), *groups[0].MatchedTopicID)
}

func TestAssembleLaneGroups_L2SwitchMergesToTarget(t *testing.T) {
	l2dec := []l2Decision{{tagID: 1, decision: "switch", targetTopicID: 9}}
	groups := assembleLaneGroups(nil, l2dec, nil)
	require.Len(t, groups, 1)
	assert.Equal(t, "l2", groups[0].Lane)
	assert.Equal(t, uint(9), *groups[0].MatchedTopicID)
	assert.Equal(t, []uint{1}, groups[0].TagIDs)
}

func TestAssembleLaneGroups_L2NewRoutedAway(t *testing.T) {
	// L2 "new" decisions are NOT merged here (they were routed to the L3 pool).
	l2dec := []l2Decision{{tagID: 1, decision: "new"}}
	groups := assembleLaneGroups(nil, l2dec, nil)
	assert.Empty(t, groups)
}

// ---- clusterLaneToTier ----

func TestClusterLaneToTier(t *testing.T) {
	assert.Equal(t, laneL1Direct, clusterLaneToTier("l1"))
	assert.Equal(t, laneL2LLM, clusterLaneToTier("l2"))
	assert.Equal(t, laneL3New, clusterLaneToTier("l3"))
	assert.Equal(t, "", clusterLaneToTier(""), "unknown lane leaves column NULL")
}

// ---- buildL2Prompt (prompt hygiene: no historical thread narratives) ----

func l2PromptFixture() ([]LaneTagAssign, map[uint]repository.BoardPersistentTopic, map[uint][]repository.TopicRecentBrief) {
	l2 := []LaneTagAssign{{
		Tag:        repository.TagInput{ID: 1, Label: "中芯国际"},
		Candidates: []LaneCandidate{{TopicID: 10, Distance: 0.22}},
	}}
	topics := map[uint]repository.BoardPersistentTopic{
		10: {
			ID:           10,
			Label:        "半导体产业链",
			Status:       repository.TopicStatusActive,
			LastSeenDate: time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC),
			HitCount:     5,
		},
	}
	briefs := map[uint][]repository.TopicRecentBrief{
		10: {{
			TopicID:    10,
			TagLabels:  []string{"半导体国产替代", "光刻机出口管制"},
			PeriodDate: time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC),
		}},
	}
	return l2, topics, briefs
}

// TestBuildL2Prompt_ExcludesThreadTitles guards the prompt-hygiene invariant:
// historical thread titles must NOT be injected into the L2 adjudication prompt.
func TestBuildL2Prompt_ExcludesThreadTitles(t *testing.T) {
	l2, topics, briefs := l2PromptFixture()
	_, user := buildL2Prompt(l2, topics, briefs)
	assert.NotContains(t, user, "半导体链全线跌停引发市场恐慌",
		"L2 prompt must not contain historical thread title narrative")
	assert.NotContains(t, user, "thread",
		"L2 prompt must not render thread entries at all")
}

// TestBuildL2Prompt_KeepsTagLabelFingerprintAndMeta guards that non-narrative
// signals (label / status / hit meta / distance / recent tag labels) remain
// injected (candidate-topic-l2-gate: tag-label fact fingerprint replaces the
// frozen section_label).
func TestBuildL2Prompt_KeepsTagLabelFingerprintAndMeta(t *testing.T) {
	l2, topics, briefs := l2PromptFixture()
	_, user := buildL2Prompt(l2, topics, briefs)
	assert.Contains(t, user, "半导体产业链", "topic label kept")
	assert.Contains(t, user, "正式", "active status label kept")
	assert.Contains(t, user, "2026-03-07", "last-seen date kept")
	assert.Contains(t, user, "累计5天", "hit count kept")
	assert.Contains(t, user, "0.220", "distance kept")
	assert.Contains(t, user, "section (2026-03-06): 半导体国产替代 / 光刻机出口管制",
		"recent tag-label fact fingerprint rendered in `section (date): tag1 / tag2` form")
}

// TestBuildL2Prompt_CandidateStrictAdjudication covers spec scenario「L2 裁决中
// 观察中话题从严判断」(candidate-topic-l2-gate D5)：system prompt SHALL 含观察期
// 从严指令，观察中候选话题的近期 tag 标签 SHALL 注入 user prompt。
func TestBuildL2Prompt_CandidateStrictAdjudication(t *testing.T) {
	l2 := []LaneTagAssign{{
		Tag:        repository.TagInput{ID: 1, Label: "伊朗代防长发表强硬言论"},
		Candidates: []LaneCandidate{{TopicID: 20, Distance: 0.08}},
	}}
	topics := map[uint]repository.BoardPersistentTopic{
		20: {
			ID:           20,
			Label:        "伊朗议长透露凌晨应急决断",
			Status:       repository.TopicStatusCandidate,
			LastSeenDate: time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC),
			HitCount:     10,
		},
	}
	briefs := map[uint][]repository.TopicRecentBrief{
		20: {{
			TopicID:    20,
			TagLabels:  []string{"伊朗议长透露哈梅内伊殉难后决断"},
			PeriodDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		}},
	}
	system, user := buildL2Prompt(l2, topics, briefs)
	// System carries the observation-period strict-keep instruction (D5).
	assert.Contains(t, system, "观察中")
	assert.Contains(t, system, "从严")
	// User renders the candidate's recent tag labels (fact fingerprint).
	assert.Contains(t, user, "状态:观察中", "candidate status rendered as 观察中")
	assert.Contains(t, user, "section (2026-03-01): 伊朗议长透露哈梅内伊殉难后决断",
		"candidate's recent tag labels injected")
}
