package service

// ReviewJudgeOutput is the structured output of the review judge LLM call.
// See design.md §4.3 and spec "分析认知循环 review judge".
type ReviewJudgeOutput struct {
	ShouldReview    bool           `json:"should_review"`
	Reason          string         `json:"reason"`
	ChangeSummary   string         `json:"change_summary"`
	AffectedContext string         `json:"affected_context"`
	Confidence      float64        `json:"confidence"`
	PositionChange  map[string]any `json:"position_change"`
}

// reviewJudgePrompt is the LLM prompt for comparing two enrichment results.
// Design §4.3: compare evolution positioning, not financial direction.
const reviewJudgePrompt = `你是一位 AI 系统质量审计员。比较同一话题的两次增强分析结果，判断话题演进定位是否发生了值得记录的迁移。

输入:
- 上次分析结果（JSON），含 analysis.position（reinforcing/turning/expanding/fading）、signals、evidence、causal_chain 等
- 本次分析结果（JSON），同上结构

四档定位含义（帮你理解迁移语义）：
- reinforcing（强化）：最新进展延续并加强了既有趋势，方向未变、力度加大
- turning（转折）：最新进展表明趋势方向发生反转，或触发质变（从缓和转紧张、从上升转下滑等）
- expanding（扩散）：影响正在传导到新领域、新主体、新地域，话题范围在扩大
- fading（衰减）：话题热度显著下降，新信号减弱或消失，不再有新的实质进展

判断标准:
- should_review=true：定位发生了迁移（from≠to），或出现了新的关键信号、因果链变化，值得记录认知更新
- should_review=false：定位未变且无实质新信息，仅措辞调整或置信度微调 → 跳过以减少噪音

position_change 填写要求:
- from：上次分析里的 analysis.position
- to：本次分析里的 analysis.position
- summary：定位怎么变了 + 凭什么。比如"停火协议被打破，局势从缓和转向紧张，触发转折"

输出严格 JSON（不要其他内容）:
{"should_review": true/false, "reason": "为什么值得/不值得复盘", "position_change": {"from": "reinforcing|turning|expanding|fading", "to": "reinforcing|turning|expanding|fading", "summary": "定位怎么变了+凭什么"}, "change_summary": "复盘说明（定位为什么变了、哪个信号触发）", "affected_context": "建议关注的上下文层 week/month/year/all", "confidence": 0.0-1.0}

---
上次分析结果:
%s

本次分析结果:
%s`
