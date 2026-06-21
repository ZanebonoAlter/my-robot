package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/jsonutil"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/topicgraph/repository"
)

func buildClusterSystemPrompt(tagCount int, existingTopics []repository.BoardPersistentTopic) string {
	base := `你是一名专业的新闻叙事分析师。你的任务是将一组事件标签按"同一叙事框架"进行分组。

分组规则：
1. 围绕同一深层主题演化的事件归入一组，形成一个叙事框架
2. 每组 2-8 个标签。单个标签应优先并入语义最近的组；仅当确实与任何标签都无关联时才独立成组，此时组名必须直接使用该标签原文，不得发挥或改写
3. 分组粒度：不是"同一事件"，而是"同一叙事框架"。一组标签应该讲述同一个更大的故事，而非仅仅是同一件事的不同报道
4. 每组给出一个叙事级标题（不超过20字）。标题必须是对该组实际包含事件的提炼与概括，禁止脱离组内事件脑补未提及的外部语境（如时间点、地点、未发生的会议、未提及的主体）
5. 必须确保每个输入标签恰好出现在一个组中

标题示例（好的，紧扣组内事件提炼）：
- 开发者 Agent 工具链进入平台化竞争
- 本地 AI 算力生态重新升温
- 企业级 AI 应用从 Demo 走向工程化落地
- 中东局势推动全球能源格局重塑

标题示例（不好的）：
- 过于具体：Codex 工具更新与第三方模型接入
- 过于具体：英伟达 6 月重磅产品发布
- 过于具体：美伊谈判进展及特朗普相关表态
- 脱离事件脑补语境：特朗普在 G7 峰会期间的盟友关系紧张（若组内只是"特朗普对美伊协议表态"等事件，不应虚构"G7 峰会""盟友关系紧张"等组内没有的要素）
- 把不相关事件强行打包：把"空军一号交付""美以关系表态""美伊谈判通牡"三个互不相关事件塞进同一组并冠以宽泛人名标题

输出要求：
1. 顶层 JSON 对象，只包含 groups 字段
2. groups 是数组，每个元素包含 group_name（字符串）、tag_ids（整数数组）和 matched_topic_id（整数，可空）
3. 只返回合法 JSON，不要 Markdown 代码块或解释文字`

	// Inject existing narrative frames so the LLM reuses them instead of
	// minting a new label for the same ongoing story (root cause A: the
	// cluster step had no memory, so labels drifted day to day).
	if len(existingTopics) > 0 {
		base += "\n\n## 该板块已有的叙事框架\n"
		base += "请优先将标签归入下列已有框架：仅当该组事件确实延续某框架的核心议题时，才把该框架 id 填入该组的 matched_topic_id，group_name 沿用或微调该框架标题。不要仅因语境沾边（如人物、地点、时间相同）就把语义不相关的事件并入。若一组标签明显属于尚未覆盖的新叙事，开新组并让 matched_topic_id 为 null。\n"
		for _, t := range existingTopics {
			statusLabel := "正式"
			if t.Status == repository.TopicStatusCandidate {
				statusLabel = "观察中"
			}
			base += fmt.Sprintf("- [id:%d] %s（状态:%s，最近命中:%s，累计%d天）\n",
				t.ID, t.Label, statusLabel,
				t.LastSeenDate.Format("2006-01-02"), t.HitCount)
		}
	}

	if tagCount > 25 {
		base += "\n6. 标签数量较多，请分成 8-15 组，合并关联性强的事件"
	} else if tagCount > 15 {
		base += "\n6. 请分成 6-12 组"
	}

	return base
}

// ClusterTags groups deduplicated tags into clusters using LLM.
// Returns cluster groups with group names and member tag IDs.
// existingTopics supplies the board's durable narrative frames so the LLM
// reuses them; each returned group carries the topic id it matched (validated
// against this set, nil when new). Pass nil/empty to cluster from scratch.
func ClusterTags(ctx context.Context, tags []repository.TagInput, existingTopics []repository.BoardPersistentTopic) ([]repository.ClusterGroup, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	// If very few tags, skip LLM and return each as its own group.
	if len(tags) <= 2 {
		groups := make([]repository.ClusterGroup, len(tags))
		for i, t := range tags {
			groups[i] = repository.ClusterGroup{
				GroupName: t.Label,
				TagIDs:    []uint{t.ID},
			}
		}
		return groups, nil
	}

	prompt := buildClusterPrompt(tags)

	temperature := 0.1
	maxTokens := 8192
	result, err := airouter.NewRouter().Chat(ctx, airouter.ChatRequest{
		Capability: airouter.CapabilityDigestPolish,
		Messages: []airouter.Message{
			{Role: "system", Content: buildClusterSystemPrompt(len(tags), existingTopics)},
			{Role: "user", Content: prompt},
		},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
		JSONMode:    true,
		JSONSchema: &airouter.JSONSchema{
			Type: "object",
			Properties: map[string]airouter.SchemaProperty{
				"groups": {
					Type: "array",
					Items: &airouter.SchemaProperty{
						Type: "object",
						Properties: map[string]airouter.SchemaProperty{
							"group_name":       {Type: "string", Description: "组名，不超过20字"},
							"tag_ids":          {Type: "array", Items: &airouter.SchemaProperty{Type: "integer"}},
							"matched_topic_id": {Type: "integer", Description: "若该组延续某个已有框架，填入该框架 id；新叙事则为 null"},
						},
						Required: []string{"group_name", "tag_ids"},
					},
				},
			},
			Required: []string{"groups"},
		},
		Metadata: map[string]any{
			"operation":       "daily_report_clustering",
			"tag_count":       len(tags),
			"existing_topics": len(existingTopics),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cluster tags AI call failed: %w", err)
	}

	logging.Infof("daily-report: cluster LLM response length=%d", len(result.Content))

	groups, err := parseClusterResponse(result.Content, tags, existingTopics)
	if err != nil {
		return nil, fmt.Errorf("parse cluster response: %w", err)
	}

	logging.Infof("daily-report: clustered %d tags into %d groups", len(tags), len(groups))
	return groups, nil
}

func buildClusterPrompt(tags []repository.TagInput) string {
	var sb strings.Builder
	sb.WriteString("## 待分组的事件标签\n\n")
	for _, t := range tags {
		fmt.Fprintf(&sb, "- [ID:%d] %s (文章数:%d", t.ID, t.Label, t.ArticleCount)
		if t.Description != "" {
			fmt.Fprintf(&sb, ", 描述:%s", t.Description)
		}
		sb.WriteString(")\n")
	}
	sb.WriteString("\n请将以上标签按核心事件分组。\n")
	return sb.String()
}

func parseClusterResponse(content string, tags []repository.TagInput, existingTopics []repository.BoardPersistentTopic) ([]repository.ClusterGroup, error) {
	content = jsonutil.SanitizeLLMJSON(content)

	// MatchedTopicID is decoded via a pointer so absent/null entries become nil.
	var raw struct {
		Groups []struct {
			GroupName      string `json:"group_name"`
			TagIDs         []uint `json:"tag_ids"`
			MatchedTopicID *uint  `json:"matched_topic_id"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse cluster JSON: %w", err)
	}

	// Validate: ensure all tags are accounted for, no duplicates, no unknown IDs.
	validTagIDs := make(map[uint]bool, len(tags))
	for _, t := range tags {
		validTagIDs[t.ID] = true
	}

	// Build the set of legal topic IDs so hallucinated matched_topic_id values
	// (an id not in the injected frame list) degrade to nil rather than corrupt
	// the dual-confirmation step.
	validTopicIDs := make(map[uint]bool, len(existingTopics))
	for _, t := range existingTopics {
		validTopicIDs[t.ID] = true
	}

	assigned := make(map[uint]bool)
	usedTopicIDs := make(map[uint]bool) // a durable topic can be claimed by at most one group
	var result []repository.ClusterGroup
	for _, g := range raw.Groups {
		if strings.TrimSpace(g.GroupName) == "" {
			continue
		}
		var validIDs []uint
		for _, id := range g.TagIDs {
			if validTagIDs[id] && !assigned[id] {
				validIDs = append(validIDs, id)
				assigned[id] = true
			}
		}
		if len(validIDs) == 0 {
			continue
		}
		var matchedID *uint
		if g.MatchedTopicID != nil && validTopicIDs[*g.MatchedTopicID] && !usedTopicIDs[*g.MatchedTopicID] {
			matchedID = g.MatchedTopicID
			usedTopicIDs[*g.MatchedTopicID] = true
		}
		result = append(result, repository.ClusterGroup{
			GroupName:      g.GroupName,
			TagIDs:         validIDs,
			MatchedTopicID: matchedID,
		})
	}

	// Assign any unassigned tags to their own group.
	for _, t := range tags {
		if !assigned[t.ID] {
			result = append(result, repository.ClusterGroup{
				GroupName: t.Label,
				TagIDs:    []uint{t.ID},
			})
			assigned[t.ID] = true
		}
	}

	return result, nil
}
