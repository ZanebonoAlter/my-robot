package service

// ReviewJudgeOutput is the structured output of the review judge LLM call.
//
// It compares the current analysis.insight_layer against the previous one and
// records cognitive updates: new findings, overturned insights, and confidence
// shifts. This replaces the old position_change/change_summary schema from the
// evolution-positioning era. Spec "分析认知对比".
//
// Invariant (business constraint #1): review NEVER writes back to table 1
// (topic_lifeline_context / news memory). It only records cognitive adoption
// in table 3 (topic_enrichment_review). News memory stays objective.
type ReviewJudgeOutput struct {
	ShouldReview    bool             `json:"should_review"`
	Reason          string           `json:"reason"`
	NewFindings     []string         `json:"new_findings"`     // 本次新出现的见解（上次没有的）
	Overturned      []string         `json:"overturned"`       // 本次推翻的旧见解
	ConfidenceShift []map[string]any `json:"confidence_shift"` // [{insight, from, to}]
	AffectedContext string           `json:"affected_context"` // week|month|year|all
	Confidence      float64          `json:"confidence"`
}

// reviewJudgePrompt is the LLM prompt comparing two enrichment analyses.
//
// Unlike the old evolution-positioning comparison (position 4 档迁移), this
// compares the insight layer directly: what's new, what's overturned, what
// shifted in certainty. Spec "分析认知对比".
const reviewJudgePrompt = `你是一位 AI 系统质量审计员。比较同一话题的两次分析结果，判断【见解层】是否发生了值得记录的认知更新。

输入：
- 上次分析结果（JSON），含 analysis.insight_layer（event_chain）或 analysis.cross_insight（theme_vein）等推演见解
- 本次分析结果（JSON），同上结构

确定性分级含义（帮你对齐 confidence_shift）：
- high：已验证的事实推论
- medium：推演·有据
- low：假设·情景
- question：提问·指出决定成败的条件

判断标准（对照两次的见解层）：
- should_review=true：出现了新的关键见解、推翻了旧见解、或见解确定性发生明显迁移 → 值得记录认知更新
- should_review=false：见解层无实质变化（仅措辞调整、证据补充）→ 跳过以减少噪音

字段填写要求：
- new_findings：本次【新出现】的见解标题（上次没有的）
- overturned：本次【推翻】的旧见解（上次有、本次否定或反转的）
- confidence_shift：见解确定性变化，每项 {"insight":"见解标题","from":"high|medium|low|question","to":"high|medium|low|question"}
- affected_context：建议关注的上下文层 week/month/year/all

输出严格 JSON（不要其他内容）：
{"should_review": true/false, "reason": "为什么值得/不值得复盘", "new_findings": ["新见解1","新见解2"], "overturned": ["被推翻的旧见解"], "confidence_shift": [{"insight":"...","from":"medium","to":"high"}], "affected_context": "week|month|year|all", "confidence": 0.0-1.0}

---
上次分析结果:
%s

本次分析结果:
%s`
