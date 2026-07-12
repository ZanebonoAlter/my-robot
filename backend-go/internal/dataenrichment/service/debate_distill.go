package service

import (
	"context"
	"encoding/json"
	"fmt"

	"syntopica-backend/internal/platform/airouter"
)

// ── Distilled types ─────────────────────────────────────────────────────────

// DistilledAgent is one agent's distilled stance from the FinGenius debate.
type DistilledAgent struct {
	Role    string `json:"role"`
	Stance  string `json:"stance"`   // "up", "down", "flat"
	Note    string `json:"note"`     // ≤15 chars summary
	RawVote string `json:"raw_vote"` // "bullish" / "bearish"
}

// DistilledDebate is the output of LLM distillation over FinGenius raw debate data.
type DistilledDebate struct {
	Agents    []DistilledAgent `json:"agents"`
	Verdict   string           `json:"verdict"`   // "up", "down", "flat"
	Consensus string           `json:"consensus"` // e.g. "4/6", "2/6 分歧"
	Votes     VoteCount        `json:"votes"`     // up/down/flat count after distillation
}

// VoteCount holds the three-stance vote tally.
type VoteCount struct {
	Up   int `json:"up"`
	Flat int `json:"flat"`
	Down int `json:"down"`
}

// ── Distiller ───────────────────────────────────────────────────────────────

// DebateDistiller uses LLM to distill FinGenius raw debate output into
// structured three-stance (up/down/flat) analysis.
type DebateDistiller struct {
	airouter   AirRouter
	capability airouter.Capability
}

// NewDebateDistiller creates a new DebateDistiller.
func NewDebateDistiller(airouter AirRouter, capability airouter.Capability) *DebateDistiller {
	return &DebateDistiller{airouter: airouter, capability: capability}
}

// Distill runs LLM distillation over FinGenius per-agent research text + vote data.
// Returns structured DistilledDebate.
func (d *DebateDistiller) Distill(ctx context.Context, sessionID string, research map[string]any, battle map[string]any) (*DistilledDebate, error) {
	researchJSON, _ := json.Marshal(research)
	battleJSON, _ := json.Marshal(battle)

	prompt := fmt.Sprintf(distillPrompt, string(researchJSON), string(battleJSON))

	resp, err := d.airouter.Chat(ctx, airouter.ChatRequest{
		Capability:  d.capability,
		Operation:   "data_enrichment.debate_distill",
		SessionID:   sessionID,
		Messages:    []airouter.Message{{Role: "user", Content: prompt}},
		Temperature: floatPtr(0.1),
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("debate distill chat: %w", err)
	}

	parsed, err := ParseJSONResponse(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("debate distill parse: %w", err)
	}

	return parseDistilledDebate(parsed)
}

// ── Prompt ──────────────────────────────────────────────────────────────────

const distillPrompt = `你是一位金融分析文本的结构化提炼专家。下面是一个多角色 AI 辩论的原始输出。

你的任务：把每个 agent 的文本分析 + 投票提炼成结构化字段。

输入包含两部分：
1. research: 每个 agent 的原始分析文本（6 个 agent 各一段长文），agent 名即 key（如 sentiment_agent, risk_agent 等）。
2. battle: 投票结果，含 final_decision（整体 bullish/bearish）、final_votes（每 agent 的 binary 票）、vote_count 等。

提炼规则：
1. **stance 三档**（up/down/flat）：final_votes[agent]=bullish → 倾向 up；bearish → down。但如果该 agent 的 research 文本语义与投票矛盾（如票 bullish 但文本说"风险积聚"），以文本为准，stance 降级为 flat。
2. **note 一句话**：从该 agent 的 research 文本提炼核心结论（≤15 字）。
3. **verdict 整体结论**：综合 6 agent stance + final_decision。up 多 → up；down 多 → down；3:3 或矛盾 → flat。
4. **consensus 共识度**："{多数 stance 票数}/6"（如 4 个 up → "4/6"；2:2:2 分歧 → "2/6 分歧"）。
5. **votes 三档统计**：统计提炼后 up/flat/down 各几票。
6. **raw_vote**：保留 FinGenius 原始投票（final_votes 中的 bullish/bearish）。

输出严格 JSON（不要其他内容）：
{"agents": [{"role": "agent 名（用 research 里的 key，如 sentiment_agent）", "stance": "up|down|flat", "note": "≤15字核心结论", "raw_vote": "bullish|bearish"}],
 "verdict": "up|down|flat",
 "consensus": "4/6 或 2/6 分歧 等",
 "votes": {"up": N, "flat": N, "down": N}}

---
research（6 agent 分析文本）:
%s

battle（投票数据）:
%s`

// ── Parser ──────────────────────────────────────────────────────────────────

func parseDistilledDebate(parsed map[string]any) (*DistilledDebate, error) {
	debate := &DistilledDebate{}

	// Parse agents.
	agentsRaw, _ := parsed["agents"].([]any)
	for _, a := range agentsRaw {
		am, ok := a.(map[string]any)
		if !ok {
			continue
		}
		role, _ := am["role"].(string)
		stance, _ := am["stance"].(string)
		note, _ := am["note"].(string)
		rawVote, _ := am["raw_vote"].(string)
		if role != "" {
			debate.Agents = append(debate.Agents, DistilledAgent{
				Role:    role,
				Stance:  stance,
				Note:    note,
				RawVote: rawVote,
			})
		}
	}

	// Parse verdict.
	debate.Verdict, _ = parsed["verdict"].(string)

	// Parse consensus.
	debate.Consensus, _ = parsed["consensus"].(string)

	// Parse votes.
	if votesRaw, ok := parsed["votes"].(map[string]any); ok {
		if u, ok := votesRaw["up"].(float64); ok {
			debate.Votes.Up = int(u)
		}
		if f, ok := votesRaw["flat"].(float64); ok {
			debate.Votes.Flat = int(f)
		}
		if d, ok := votesRaw["down"].(float64); ok {
			debate.Votes.Down = int(d)
		}
	}

	return debate, nil
}
