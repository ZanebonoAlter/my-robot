package service

import (
	"context"
	"fmt"

	"syntopica-backend/internal/platform/airouter"
)

// Lens is a concrete problem-style analysis viewpoint (e.g. "美国为何在对华
// 芯片政策上反复横跳", NOT an abstract tag like "博弈论"). Spec "分析视角候选
// 与选择"（模式丙：agent 提候选 + 用户选）。
type Lens struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// LensSource proposes concrete lens candidates for a topic given its classified
// form. It is the extension point for viewpoint sources: the first
// implementation is AgentLensSource (LLM-generated); future external sources
// (video commentators, research reports) implement the same interface and can
// be swapped in without touching orchestration. Spec "视角来源可扩展".
type LensSource interface {
	Propose(ctx context.Context, ictx interpretContext, form string) ([]Lens, error)
}

// lensProposePrompt is the LLM prompt for proposing analysis viewpoints.
// Enforces concrete problem-style lenses and rejects abstract tags.
const lensProposePrompt = `你是一位资深产业分析师。下面是一个持久话题的演进脉络，话题形态已判断为「%s」。

你的任务：针对这个话题，提出【具体可讨论的视角候选】——必须是【问题式、具体】的视角，不是抽象标签。
- 好的视角示例："美国为何在对华芯片政策上反复横跳"、"油价这轮上涨能不能持续"、"欧盟AI法案会否引发中美监管竞赛"
- 禁止的视角（抽象标签）："博弈论"、"宏观经济"、"地缘政治"

要求：
- 至少 2 个视角候选
- 每个视角给出视角名（问题式）+ 一句话说明（为什么值得从这个视角看）
- 视角之间要有差异化，覆盖不同维度

输出严格 JSON：
{"lens_candidates": [{"name": "具体问题式视角", "description": "为什么值得从这个视角看"}]}`

// AgentLensSource generates lens candidates via an LLM call. It is the default
// LensSource implementation bound to the orchestrator's airouter + capability.
type AgentLensSource struct {
	airouter   AirRouter
	capability airouter.Capability
}

// NewAgentLensSource creates an AgentLensSource bound to the shared airouter.
func NewAgentLensSource(airouter AirRouter, capability airouter.Capability) *AgentLensSource {
	return &AgentLensSource{airouter: airouter, capability: capability}
}

// Propose asks the LLM for ≥2 concrete problem-style lens candidates given the
// topic's classified form. Uses the same Operation as interpret (viewpoint
// proposal belongs to the interpreter phase).
func (s *AgentLensSource) Propose(ctx context.Context, ictx interpretContext, form string) ([]Lens, error) {
	prompt := fmt.Sprintf(lensProposePrompt, form) + "\n\n---\n"
	if ictx.ContextText != "" {
		prompt += "分层新闻上下文:\n" + ictx.ContextText + "\n\n"
	}
	prompt += "话题演进脉络:\n" + ictx.LifelineText

	resp, err := s.airouter.Chat(ctx, airouter.ChatRequest{
		Capability:  s.capability,
		Operation:   "data_enrichment.interpret",
		SessionID:   ictx.SessionID,
		Messages:    []airouter.Message{{Role: "user", Content: prompt}},
		Temperature: floatPtr(0.3),
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("lens propose chat: %w", err)
	}

	parsed, err := ParseJSONResponse(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("lens propose parse: %w", err)
	}

	raw, ok := parsed["lens_candidates"].([]any)
	if !ok {
		return nil, fmt.Errorf("lens propose: missing or invalid 'lens_candidates' field")
	}

	lenses := make([]Lens, 0, len(raw))
	for _, l := range raw {
		lm, ok := l.(map[string]any)
		if !ok {
			continue
		}
		name, _ := lm["name"].(string)
		desc, _ := lm["description"].(string)
		if name != "" {
			lenses = append(lenses, Lens{Name: name, Description: desc})
		}
	}

	if len(lenses) < 2 {
		return nil, fmt.Errorf("lens propose: need >=2 lens candidates, got %d", len(lenses))
	}
	return lenses, nil
}
