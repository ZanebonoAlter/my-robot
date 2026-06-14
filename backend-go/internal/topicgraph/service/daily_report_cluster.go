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

func buildClusterSystemPrompt(tagCount int) string {
	base := `你是一名专业的新闻叙事分析师。你的任务是将一组事件标签按"同一叙事框架"进行分组。

分组规则：
1. 围绕同一深层主题演化的事件归入一组，形成一个叙事框架
2. 每组 2-8 个标签；如果某个标签找不到同类，可以单独成组
3. 分组粒度：不是"同一事件"，而是"同一叙事框架"。一组标签应该讲述同一个更大的故事，而非仅仅是同一件事的不同报道
4. 每组给出一个叙事级标题（不超过20字），应该是跨事件的解释性判断，而非事件描述
5. 必须确保每个输入标签恰好出现在一个组中

标题示例（好的）:
- 开发者 Agent 工具链进入平台化竞争
- 本地 AI 算力生态重新升温
- 企业级 AI 应用从 Demo 走向工程化落地
- 中东局势推动全球能源格局重塑

标题示例（不好的，太具体）:
- Codex 工具更新与第三方模型接入
- 英伟达 6 月重磅产品发布
- 美伊谈判进展及特朗普相关表态

输出要求：
1. 顶层 JSON 对象，只包含 groups 字段
2. groups 是数组，每个元素包含 group_name（字符串）和 tag_ids（整数数组）
3. 只返回合法 JSON，不要 Markdown 代码块或解释文字`

	if tagCount > 25 {
		base += "\n6. 标签数量较多，请分成 8-15 组，合并关联性强的事件"
	} else if tagCount > 15 {
		base += "\n6. 请分成 6-12 组"
	}

	return base
}

// ClusterTags groups deduplicated tags into clusters using LLM.
// Returns cluster groups with group names and member tag IDs.
func ClusterTags(ctx context.Context, tags []repository.TagInput) ([]repository.ClusterGroup, error) {
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
			{Role: "system", Content: buildClusterSystemPrompt(len(tags))},
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
							"group_name": {Type: "string", Description: "组名，不超过20字"},
							"tag_ids":    {Type: "array", Items: &airouter.SchemaProperty{Type: "integer"}},
						},
						Required: []string{"group_name", "tag_ids"},
					},
				},
			},
			Required: []string{"groups"},
		},
		Metadata: map[string]any{
			"operation": "daily_report_clustering",
			"tag_count": len(tags),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cluster tags AI call failed: %w", err)
	}

	logging.Infof("daily-report: cluster LLM response length=%d", len(result.Content))

	groups, err := parseClusterResponse(result.Content, tags)
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

func parseClusterResponse(content string, tags []repository.TagInput) ([]repository.ClusterGroup, error) {
	content = jsonutil.SanitizeLLMJSON(content)

	var raw struct {
		Groups []repository.ClusterGroup `json:"groups"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse cluster JSON: %w", err)
	}

	// Validate: ensure all tags are accounted for, no duplicates, no unknown IDs.
	validTagIDs := make(map[uint]bool, len(tags))
	for _, t := range tags {
		validTagIDs[t.ID] = true
	}

	assigned := make(map[uint]bool)
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
		result = append(result, repository.ClusterGroup{
			GroupName: g.GroupName,
			TagIDs:    validIDs,
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
