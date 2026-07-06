package service

// ReviewJudgeOutput is the structured output of the review judge LLM call.
// See design.md §4.3 and spec "分析认知循环 review judge".
type ReviewJudgeOutput struct {
	ShouldReview     bool    `json:"should_review"`
	Reason           string  `json:"reason"`
	DeviationSummary string  `json:"deviation_summary"`
	AffectedContext  string  `json:"affected_context"`
	Confidence       float64 `json:"confidence"`
}

// reviewJudgePrompt is the LLM prompt for comparing two enrichment results.
// Design §4.3: semi-automatic comparison. Output JSON.
const reviewJudgePrompt = `你是一位 AI 系统质量审计员。比较同一话题的两次增强分析结果，判断认知是否需要更新。

输入:
- 上次分析结果（JSON）
- 本次分析结果（JSON）

判断标准:
- should_review=true: 核心判断发生反转、演进阶段变化、或出现了新的因果链
- should_review=false: 仅置信度微调、措辞变化但判断一致、或无实质新信息

输出严格 JSON（不要其他内容）:
{"should_review": true/false, "reason": "为什么值得/不值得生成新的认知记录", "deviation_summary": "核心判断变化摘要", "affected_context": "建议关注的上下文层 week/month/year/all", "confidence": 0.0-1.0}

---
上次分析结果:
%s

本次分析结果:
%s`
