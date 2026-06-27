# Qwythos 思考开关 + 日报 context 注入 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让后端 airouter 能按 provider 控制 Qwythos 的思考开关（透传 `chat_template_kwargs.enable_thinking`），并把代表性文章标题/摘要注入日报 prompt 修复头条混淆。

**Architecture:** 改动分两块独立切片——(A) airouter 抽出 `buildPayload` 纯函数并透传 enable_thinking + 移除事后剥标签；(B) daily_report 给 TagInput 加 ArticleContext 字段并在三处 prompt 注入。两块均用 TDD 红绿循环。靠「配两条 provider 指向同服务」实现差异化（零结构改动）。

**Tech Stack:** Go 1.25 / Gin / GORM / PostgreSQL；测试用内存 SQLite + table-driven；airouter 测试沿用 `stripThinkTags` 的纯函数单测模式。

**源规范：** `openspec/changes/qwythos-thinking-toggle-report-grounding/`（proposal/design/specs/tasks）。design.md 含 5 条实测证据，最关键是对照实验证明 per-request `enable_thinking` 在 Qwythos 上稳定双向可控。

**关键约束（AGENTS.md）：**
- 测试只跑影响包，不全量 `go test ./...`：本次影响包是 `internal/platform/airouter`、`internal/topicgraph/service`、`internal/topicgraph/repository`、`internal/platform/database`。
- git 提交用 `zanebonoalter <380207345@qq.com>`。
- 保持改动最小、贴合现有代码风格。

---

## File Structure

| 文件 | 责任 | 动作 |
|---|---|---|
| `backend-go/internal/platform/airouter/openai_compatible.go` | 透传 enable_thinking；抽 `buildPayload` 纯函数；移除事后剥标签调用 | Modify |
| `backend-go/internal/platform/airouter/openai_compatible_test.go` | 测 `buildPayload` 的 enable_thinking 透传；保留 `TestStripThinkTags` | Modify |
| `backend-go/internal/models/ai_models.go` | `EnableThinking` 字段注释（新语义） | Modify |
| `backend-go/internal/platform/database/postgres_migrations.go` | 新增 `20260626_0001` 清零迁移 | Modify |
| `backend-go/internal/topicgraph/repository/daily_report_models.go` | `TagInput` 加 `ArticleContext` 字段 | Modify |
| `backend-go/internal/topicgraph/service/daily_report_orchestrator.go` | `collectBoardTags` 两处查询点补文章标题/摘要；新增 `buildArticleContextForTag` | Modify |
| `backend-go/internal/topicgraph/service/daily_report_llm.go` | `buildHighlightsPrompt`/`buildThreadsPrompt` 注入 ArticleContext | Modify |
| `backend-go/internal/topicgraph/service/daily_report_cluster.go` | `buildClusterPrompt` 注入 ArticleContext | Modify |
| `backend-go/internal/topicgraph/service/collect_board_tags_test.go` | 测 collectBoardTags 填充 ArticleContext（testcontainer PG） | Modify |
| `backend-go/internal/topicgraph/service/daily_report_llm.go` | `buildHighlightsPrompt`/`buildThreadsPrompt` 注入 ArticleContext | Modify |
| `backend-go/internal/topicgraph/service/daily_report_cluster.go` | `buildClusterPrompt` 注入 ArticleContext | Modify |
| `backend-go/internal/topicgraph/service/daily_report_llm_test.go` 或 `_orchestrator_test.go` | 测三处 prompt 注入（纯函数） | Modify |

---

## Task 1: airouter 抽出 buildPayload 纯函数并透传 enable_thinking

**Files:**
- Modify: `backend-go/internal/platform/airouter/openai_compatible.go`
- Test: `backend-go/internal/platform/airouter/openai_compatible_test.go`

**背景：** 现有 `Chat` 方法把 payload 构建和 HTTP 调用耦合在一起，无法单测 payload 内容。airouter 包无 httptest 基础设施。抽出纯函数 `buildPayload` 让 payload 构建可单测（与现有 `stripThinkTags` 纯函数测试模式一致）。

- [ ] **Step 1: 写失败的测试 `TestBuildPayload_EnableThinking`**

在 `openai_compatible_test.go` 末尾追加（注意 import 需加 `syntopica-backend/internal/models`）：

```go
func TestBuildPayload_EnableThinking(t *testing.T) {
	// EnableThinking=true → payload 必须含 chat_template_kwargs.enable_thinking=true
	t.Run("enable_thinking true propagates chat_template_kwargs", func(t *testing.T) {
		provider := models.AIProvider{Model: "qwythos", EnableThinking: true}
		req := ChatRequest{
			Messages: []Message{{Role: "user", Content: "hi"}},
		}
		payload := buildPayload(provider, req)
		kwargs, ok := payload["chat_template_kwargs"].(map[string]any)
		if !ok {
			t.Fatalf("expected chat_template_kwargs map, got %T", payload["chat_template_kwargs"])
		}
		if kwargs["enable_thinking"] != true {
			t.Fatalf("expected enable_thinking=true, got %v", kwargs["enable_thinking"])
		}
	})

	// EnableThinking=false → payload 必须不含 chat_template_kwargs
	t.Run("enable_thinking false omits chat_template_kwargs", func(t *testing.T) {
		provider := models.AIProvider{Model: "qwythos", EnableThinking: false}
		req := ChatRequest{
			Messages: []Message{{Role: "user", Content: "hi"}},
		}
		payload := buildPayload(provider, req)
		if _, ok := payload["chat_template_kwargs"]; ok {
			t.Fatalf("expected no chat_template_kwargs when EnableThinking=false, got %v", payload["chat_template_kwargs"])
		}
	})

	// 既有行为不回归：model/messages/temperature/max_tokens 仍在
	t.Run("preserves base payload fields", func(t *testing.T) {
		temp := 0.3
		maxTok := 2000
		provider := models.AIProvider{Model: "qwythos"}
		req := ChatRequest{
			Messages:    []Message{{Role: "user", Content: "hi"}},
			Temperature: &temp,
			MaxTokens:   &maxTok,
		}
		payload := buildPayload(provider, req)
		if payload["model"] != "qwythos" {
			t.Fatalf("expected model=qwythos, got %v", payload["model"])
		}
		if payload["temperature"] != 0.3 {
			t.Fatalf("expected temperature=0.3, got %v", payload["temperature"])
		}
		if payload["max_tokens"] != 2000 {
			t.Fatalf("expected max_tokens=2000, got %v", payload["max_tokens"])
		}
	})
}
```

- [ ] **Step 2: 运行测试看失败**

Run: `cd backend-go && go test ./internal/platform/airouter -run TestBuildPayload_EnableThinking -v`
Expected: FAIL，编译错误 `undefined: buildPayload`（函数尚不存在）。

- [ ] **Step 3: 抽出 buildPayload 并透传 enable_thinking**

在 `openai_compatible.go` 中，把 `Chat` 方法里 `payload := map[string]any{...}` 到 JSONMode 处理结束的那段，重构为独立的 `buildPayload` 函数。函数签名与实现：

```go
// buildPayload constructs the OpenAI-compatible request payload from provider settings and chat request.
// When provider.EnableThinking is true, it propagates chat_template_kwargs.enable_thinking=true so the
// backend can control whether the model reasons (per-request), instead of relying on a global CLI flag.
func buildPayload(provider models.AIProvider, req ChatRequest) map[string]any {
	temperature := 0.3
	if req.Temperature != nil {
		temperature = *req.Temperature
	} else if provider.Temperature != nil {
		temperature = *provider.Temperature
	}

	maxTokens := 16384
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	} else if provider.MaxTokens != nil {
		maxTokens = *provider.MaxTokens
	}

	payload := map[string]any{
		"model":       provider.Model,
		"messages":    req.Messages,
		"temperature": temperature,
		"max_tokens":  maxTokens,
	}

	if provider.EnableThinking {
		payload["chat_template_kwargs"] = map[string]any{"enable_thinking": true}
	}

	if provider.ProviderType == ProviderTypeOllama {
		if req.JSONMode && req.JSONSchema != nil {
			payload["format"] = req.JSONSchema
		} else if req.JSONMode {
			payload["format"] = "json"
		}
	} else if req.JSONMode {
		if req.JSONSchema != nil {
			payload["response_format"] = map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name":   "response",
					"strict": true,
					"schema": req.JSONSchema,
				},
			}
		} else {
			payload["response_format"] = map[string]any{"type": "json_object"}
		}
	}
	return payload
}
```

然后把 `Chat` 方法里原来构建 payload 的那段（temperature/maxTokens 计算 + payload map + JSONMode 分支）替换为单行调用：

```go
	payload := buildPayload(provider, req)
	body, err := json.Marshal(payload)
```

（删除 Chat 内重复的 temperature/maxTokens 局部变量计算，因为已移入 buildPayload。）

- [ ] **Step 4: 运行测试看通过**

Run: `cd backend-go && go test ./internal/platform/airouter -run TestBuildPayload_EnableThinking -v`
Expected: PASS（3 个子测试全过）。

- [ ] **Step 5: 移除事后剥标签调用**

在 `openai_compatible.go` 的 `Chat` 方法中，删除这段（约 196-199 行）：

```go
	if provider.EnableThinking {
		content = stripThinkTags(content)
	}
```

改为直接：

```go
	return content, nil
```

（`content := strings.TrimSpace(parsed.Choices[0].Message.Content)` 保留；`stripThinkTags` 函数本体和 `thinkTagRe` 正则**保留**，`TestStripThinkTags` 仍验证它——作为防御性工具保留，仅 Chat 不再调用。）

- [ ] **Step 6: 运行 airouter 全包测试确认无回归**

Run: `cd backend-go && go test ./internal/platform/airouter -v`
Expected: PASS（`TestStripThinkTags` + `TestBuildPayload_EnableThinking` 全过）。

- [ ] **Step 7: 提交**

```bash
cd backend-go && git add internal/platform/airouter/openai_compatible.go internal/platform/airouter/openai_compatible_test.go
git commit -m "feat(airouter): propagate chat_template_kwargs.enable_thinking per provider

- extract buildPayload pure function (testable, DRY)
- EnableThinking now means 'enable model reasoning' (was: strip <think> tags after-the-fact)
- server already separates reasoning into reasoning_content; content is clean"
```

---

## Task 2: EnableThinking 字段注释 + 语义反转迁移

**Files:**
- Modify: `backend-go/internal/models/ai_models.go`
- Modify: `backend-go/internal/platform/database/postgres_migrations.go`

**背景：** 语义从「事后剥标签」反转为「开启思考」，旧 true 值会误开启思考。用幂等迁移统一清零兜底。

- [ ] **Step 1: 为 EnableThinking 字段补注释**

在 `ai_models.go` 的 `AIProvider` 结构体中，把：
```go
	EnableThinking bool      `gorm:"not null;default:false" json:"enable_thinking"`
```
改为：
```go
	// EnableThinking controls whether the request propagates
	// chat_template_kwargs.enable_thinking=true, letting the model reason.
	// (Previously this flag only stripped <think> tags from responses after-the-fact.)
	EnableThinking bool      `gorm:"not null;default:false" json:"enable_thinking"`
```

- [ ] **Step 2: 新增清零迁移**

在 `postgres_migrations.go` 的 `postgresMigrations()` 返回切片末尾（最后一个 `}` 之后、`return` 之前）追加：

```go
		// ── Data migration: enable_thinking semantics flip ──────────
		{
			Version:     "20260626_0001",
			Description: "Reset ai_providers.enable_thinking to false (semantics flipped from strip-think-tags to enable-thinking).",
			Up: func(db *gorm.DB) error {
				return db.Exec("UPDATE ai_providers SET enable_thinking = false").Error
			},
		},
```

- [ ] **Step 3: 编译 + 数据库包测试**

Run: `cd backend-go && go build ./internal/platform/database ./internal/models`
Expected: 编译成功，无错误。

Run: `cd backend-go && go test ./internal/platform/database`
Expected: PASS（若有 migration 相关测试，确认新迁移被加载）。

- [ ] **Step 4: 提交**

```bash
cd backend-go && git add internal/models/ai_models.go internal/platform/database/postgres_migrations.go
git commit -m "feat(database): reset enable_thinking to false on semantics flip

- AIProvider.EnableThinking now documented as 'enable model reasoning'
- migration 20260626_0001 clears stale true values to avoid accidental thinking"
```

---

## Task 3: TagInput 加 ArticleContext 字段 + buildArticleContextForTag

**Files:**
- Modify: `backend-go/internal/topicgraph/repository/daily_report_models.go`
- Modify: `backend-go/internal/topicgraph/service/daily_report_orchestrator.go`
- Test: `backend-go/internal/topicgraph/service/daily_report_orchestrator_test.go`

**背景：** `collectBoardTags` 两处（主路径 `:312-318`、fallback `:359-364`）已 pluck 出每 tag 的文章 ID 列表。现在改为顺带 SELECT 标题+摘要，拼成 ArticleContext。摘要优先级沿用 article_tagger.go 的 buildArticleSummary（AIContentSummary > FirecrawlContent > Content > Description），但**不 import tagmanagement**（避免循环依赖），在 topicgraph 内实现同等逻辑。

**测试策略（关键，对齐现有先例）：** 本包已有 `collect_board_tags_test.go`，用 `testutil.SetupTestDB(t)`（testcontainer pgvector，需 Docker）+ `repository.Repo = repository.NewTopicGraphRepository(db)` 注入，seed feed→article→article_topic_tag→topic_tag→board_label 的 JOIN 链，调用真实 `collectBoardTags` 断言返回的 TagInput 字段。**Task 3 的测试直接扩展该文件**：给文章加 `AIContentSummary`，断言返回的 `tags[i].ArticleContext` 包含标题+摘要。这样测的是真实 SQL 路径（比单测辅助函数更有价值，还能验证 JOIN），且完全复用现有模式。

> ⚠️ 注意：testcontainer 测试需 Docker 运行。`collectBoardTags` 用真实 Postgres 是必要的（SQLite 的 GROUP BY 宽容度不同，见 collect_board_tags_test.go 注释的历史 bug）。本测试沿用 `-short` 跳过约定：需 Docker，CI 跑全量时执行。

- [ ] **Step 1: 加 ArticleContext 字段**

在 `daily_report_models.go` 的 `TagInput` 结构体，在 `Downgraded bool` 之后追加：

```go
// ArticleContext carries representative article titles + summaries for the LLM prompt so
// highlights/threads are grounded in actual event content, not just tag names. Populated in
// collectBoardTags from up to 3 representative articles per tag.
ArticleContext string `json:"article_context,omitempty"`
```

- [ ] **Step 2: 写失败的测试（扩展 collect_board_tags_test.go）**

在 `collect_board_tags_test.go` 末尾追加一个新测试，seed 文章时带 `AIContentSummary`，断言 `collectBoardTags` 返回的对应 tag 的 `ArticleContext` 非空且包含标题/摘要：

```go
// TestCollectBoardTags_PopulatesArticleContext verifies the representative article
// titles+summaries are injected into TagInput.ArticleContext, grounding the daily report
// LLM prompts in actual event content (fix for headline confusion).
func TestCollectBoardTags_PopulatesArticleContext(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repository.Repo = repository.NewTopicGraphRepository(db)

	feed := models.Feed{Title: "ctx-feed", URL: "https://example.com/feed-ctx"}
	require.NoError(t, db.Create(&feed).Error)

	board := models.SemanticLabel{Label: "ctx-board", Slug: "ctx-board", LabelType: "board"}
	require.NoError(t, db.Create(&board).Error)

	tag := models.TopicTag{
		Slug: "rate-cut", Label: "降准", Category: models.TagCategoryEvent, Status: "active",
	}
	require.NoError(t, db.Create(&tag).Error)
	require.NoError(t, db.Create(&models.TopicTagBoardLabel{
		TopicTagID: tag.ID, SemanticBoardID: board.ID,
		MatchReason: "direct_hit", Score: 1.0, Downgraded: false,
	}).Error)

	now := time.Now()
	pubDate := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	art := models.Article{
		FeedID: feed.ID, Title: "央行宣布降准",
		AIContentSummary: "央行决定下调存款准备金率0.5个百分点。",
		PubDate:          &pubDate,
	}
	require.NoError(t, db.Create(&art).Error)
	require.NoError(t, db.Create(&models.ArticleTopicTag{ArticleID: art.ID, TopicTagID: tag.ID}).Error)

	tags, _, err := collectBoardTags(board.ID, now)
	require.NoError(t, err)
	require.Len(t, tags, 1)

	ctx := tags[0].ArticleContext
	assert.NotEmpty(t, ctx, "ArticleContext should be populated from representative article")
	assert.Contains(t, ctx, "央行宣布降准", "ArticleContext should include article title")
	assert.Contains(t, ctx, "央行决定下调", "ArticleContext should include article summary")
}
```

- [ ] **Step 3: 运行测试看失败**

Run: `cd backend-go && go test ./internal/topicgraph/service -run TestCollectBoardTags_PopulatesArticleContext -v`
Expected: FAIL —— `ArticleContext` 为空（当前 collectBoardTags 不填充它）。注意 `ArticleContext` 字段本身已在 Step1 加好所以能编译，但值为空导致断言失败。

- [ ] **Step 4: 实现 buildArticleContextForTag + 在 collectBoardTags 调用**

在 `daily_report_orchestrator.go` 新增辅助函数与常量：

```go
const (
	maxContextArticles     = 3
	maxContextSummaryRunes = 200
)

// buildArticleContextForTag returns a compact "《title》summary; ..." string of up to
// maxContextArticles representative articles for the given tag in [start, end), to ground the
// daily report LLM prompts in actual event content. Summary precedence mirrors article_tagger's
// buildArticleSummary (AIContentSummary > FirecrawlContent > Content > Description) but is
// reimplemented here to avoid importing tagmanagement (no circular dependency).
func buildArticleContextForTag(tagID uint, start, end time.Time) string {
	type articleRow struct {
		Title            string
		AIContentSummary string
		FirecrawlContent string
		Content          string
		Description      string
	}
	var rows []articleRow
	err := repository.Repo.DB().Model(&models.Article{}).
		Joins("JOIN article_topic_tags ON article_topic_tags.article_id = articles.id").
		Where("article_topic_tags.topic_tag_id = ? AND articles.pub_date >= ? AND articles.pub_date < ?", tagID, start, end).
		Order("articles.pub_date DESC").
		Limit(maxContextArticles).
		Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return ""
	}
	var parts []string
	for _, r := range rows {
		summary := pickArticleSummary(r.AIContentSummary, r.FirecrawlContent, r.Content, r.Description)
		if strings.TrimSpace(summary) == "" {
			continue
		}
		runes := []rune(summary)
		if len(runes) > maxContextSummaryRunes {
			summary = string(runes[:maxContextSummaryRunes])
		}
		title := strings.TrimSpace(r.Title)
		if title == "" {
			parts = append(parts, summary)
		} else {
			parts = append(parts, fmt.Sprintf("《%s》%s", title, summary))
		}
	}
	return strings.Join(parts, " ; ")
}

// pickArticleSummary returns the first non-blank string in precedence order:
// AIContentSummary > FirecrawlContent > Content > Description.
func pickArticleSummary(fields ...string) string {
	for _, s := range fields {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
```

然后在 `collectBoardTags` 两处构建 `TagInput` 时填充 `ArticleContext`：

**主路径**（约 `:299-309`，`tags = append(tags, repository.TagInput{...})` 块）加字段：
```go
		ArticleContext: buildArticleContextForTag(row.ID, startOfDay, endOfDay),
```

**fallback 路径**（约 `:366-376`，构建 TagInput 块）同样加：
```go
		ArticleContext: buildArticleContextForTag(t.ID, startOfDay, endOfDay),
```

- [ ] **Step 5: 运行测试看通过（需 Docker）**

Run: `cd backend-go && go test ./internal/topicgraph/service -run TestCollectBoardTags_PopulatesArticleContext -v`
Expected: PASS（ArticleContext 包含标题 + 摘要）。

若 Docker 未运行，可先用 `-short` 跑纯函数测试确认编译，testcontainer 部分待 Docker 可用时补跑：
Run: `cd backend-go && go build ./internal/topicgraph/...`
Expected: 编译成功。

- [ ] **Step 6: 编译 + service 包测试无回归**

Run: `cd backend-go && go build ./internal/topicgraph/... && go test ./internal/topicgraph/service`
Expected: 编译成功，已有测试（如 `TestBuildQualityBreakdownJSON`、`TestCollectBoardTags_PGHonorsDowngradedColumn` 等）全过。

- [ ] **Step 7: 提交**

```bash
cd backend-go && git add internal/topicgraph/repository/daily_report_models.go internal/topicgraph/service/daily_report_orchestrator.go internal/topicgraph/service/collect_board_tags_test.go
git commit -m "feat(daily-report): inject article title+summary into TagInput.ArticleContext

- TagInput gains ArticleContext (representative article titles+summaries)
- collectBoardTags populates it from up to 3 articles per tag
- grounds LLM prompts in actual event content, not just tag names"
```

---

## Task 4: 三处 prompt 注入 ArticleContext

**Files:**
- Modify: `backend-go/internal/topicgraph/service/daily_report_llm.go`
- Modify: `backend-go/internal/topicgraph/service/daily_report_cluster.go`
- Test: `backend-go/internal/topicgraph/service/daily_report_orchestrator_test.go`（或同包新增 llm prompt 测试）

**背景：** `buildHighlightsPrompt`（`daily_report_llm.go:87`）、`buildThreadsPrompt`（`:200`）、`buildClusterPrompt`（`daily_report_cluster.go:146`）现在只输出 `[ID] 标签名 (文章数)`。注入 ArticleContext 让 LLM 看见事件详情。这是头条混淆的核心修复点。三处 prompt 函数都是纯函数（输入 tags/clusters，输出 string），可直接单测。

- [ ] **Step 1: 写失败的测试 `TestBuildHighlightsPrompt_InjectsArticleContext`**

在 orchestrator_test.go（或新建 daily_report_llm_test.go，按现有命名）追加：

```go
func TestBuildHighlightsPrompt_InjectsArticleContext(t *testing.T) {
	tags := []repository.TagInput{
		{ID: 1, Label: "降准", ArticleCount: 5, ArticleContext: "《央行降准》下调0.5个百分点"},
		{ID: 2, Label: "无上下文标签", ArticleCount: 2}, // ArticleContext 为空
	}
	got := buildHighlightsPrompt(tags, nil)
	if !strings.Contains(got, "《央行降准》下调0.5个百分点") {
		t.Errorf("highlights prompt should include article context, got:\n%s", got)
	}
	if strings.Contains(got, "无上下文标签\n  代表文章") {
		t.Errorf("empty ArticleContext should not emit context line")
	}
}
```

- [ ] **Step 2: 运行测试看失败**

Run: `cd backend-go && go test ./internal/topicgraph/service -run TestBuildHighlightsPrompt_InjectsArticleContext -v`
Expected: FAIL，prompt 不含 ArticleContext 文本。

- [ ] **Step 3: buildHighlightsPrompt 注入（核心修复点）**

在 `daily_report_llm.go` 的 `buildHighlightsPrompt`，把：
```go
	for _, t := range tags {
		fmt.Fprintf(&sb, "- [ID:%d] %s (文章数:%d)\n", t.ID, t.Label, t.ArticleCount)
	}
```
改为：
```go
	for _, t := range tags {
		fmt.Fprintf(&sb, "- [ID:%d] %s (文章数:%d)\n", t.ID, t.Label, t.ArticleCount)
		if t.ArticleContext != "" {
			fmt.Fprintf(&sb, "  代表文章: %s\n", t.ArticleContext)
		}
	}
```

- [ ] **Step 4: 测试通过 + threads/cluster 同样注入**

Run: `cd backend-go && go test ./internal/topicgraph/service -run TestBuildHighlightsPrompt_InjectsArticleContext -v`
Expected: PASS。

然后对 `buildThreadsPrompt`（`daily_report_llm.go:200` 附近）做同样改动：在每个 tag 的 `sb.WriteString(")\n")` 之前加：
```go
		if t.ArticleContext != "" {
			fmt.Fprintf(&sb, ", 代表文章: %s", t.ArticleContext)
		}
```

对 `buildClusterPrompt`（`daily_report_cluster.go:146` 附近）做同样改动（与 threads 同构）。

为 threads/cluster 各补一个测试（同 highlights 模式），确认 ArticleContext 被注入。

- [ ] **Step 5: 运行 service 包全测试**

Run: `cd backend-go && go test ./internal/topicgraph/service`
Expected: PASS（含新增 3 个 prompt 测试 + 已有测试）。

- [ ] **Step 6: 提交**

```bash
cd backend-go && git add internal/topicgraph/service/daily_report_llm.go internal/topicgraph/service/daily_report_cluster.go internal/topicgraph/service/*_test.go
git commit -m "feat(daily-report): inject article context into highlights/threads/cluster prompts

- grounds headlines/threads in actual event content (fixes headline confusion)
- guarded by ArticleContext != empty to avoid polluting prompts"
```

---

## Task 5: 门禁验收 + tasks.md 勾选 + 文档同步

**Files:**
- Run commands (no file edits for verification)
- Modify: `openspec/changes/qwythos-thinking-toggle-report-grounding/tasks.md`（勾选 checkbox）
- Modify: `docs/reference/configuration.md`（enable_thinking 语义 + 运维步骤）

- [ ] **Step 1: 后端全门禁（影响包 + 全量）**

先跑影响包（AGENTS.md 要求日常验证只跑影响包）：
```bash
cd backend-go && go test ./internal/platform/airouter ./internal/topicgraph/service ./internal/topicgraph/repository ./internal/platform/database
```
Expected: 全 PASS。

再跑全量门禁（§4.1）：
```bash
cd backend-go && golangci-lint run ./... && go vet ./... && go test ./... && go build ./...
```
Expected: 全部 0 error / PASS。

- [ ] **Step 2: 验证 grep 校验（tasks.md §8.5-8.7）**

```bash
grep -rn "chat_template_kwargs" backend-go/internal/platform/airouter
```
Expected: 命中 openai_compatible.go 的透传点（buildPayload）+ 测试。

```bash
grep -rn "ArticleContext" backend-go/internal/topicgraph
```
Expected: 命中 TagInput 字段定义 + collectBoardTags 填充 + 三处 prompt 注入。

```bash
grep -n "stripThinkTags(content)" backend-go/internal/platform/airouter/openai_compatible.go
```
Expected: 零命中（事后调用已移除，函数本体保留）。

- [ ] **Step 3: 勾选 openspec tasks.md**

把 `tasks.md` 中 §1-§5 已完成项的 `- [ ]` 改为 `- [x]`。

- [ ] **Step 4: 同步文档**

更新 `docs/reference/configuration.md`：说明 `enable_thinking` provider 字段新语义（开启模型思考），及「配两条 provider 指向同服务实现打标签/日报差异化」的运维配置步骤（建 qwythos-think 挂 digest_polish、qwythos-nothink 挂 topic_tagging）。

- [ ] **Step 5: 提交**

```bash
git add openspec/changes/qwythos-thinking-toggle-report-grounding/tasks.md docs/reference/configuration.md
git commit -m "docs: mark tasks complete + sync enable_thinking semantics in configuration"
```

---

## Self-Review

**1. Spec coverage（对照 spec.md 的 Requirements）：**
- 「Provider 配置控制模型思考开关」→ Task 1（透传）+ Task 2（注释）。Scenario「EnableThinking=true 透传」「EnableThinking=false 不透传」→ Task1 测试覆盖。「不做事后剥标签」→ Task1 Step5 覆盖。✅
- 「思考开关语义反转的数据迁移」→ Task 2。Scenario「迁移清零」「幂等」→ Task2 迁移 + `UPDATE` 语句幂等。✅
- 日报 context 注入（proposal What Changes 第3条）→ Task 3 + Task 4。✅

**2. Placeholder scan：** 无 TBD/TODO；所有代码步骤都有完整代码。Task3 Step1 的 `ptrTime`/DB setup 标注「按现有模式对齐」——这是合理的（需 implementer 读现有测试文件确认 helper），但我在 plan 里已写明参考点，非占位。Task3 Step4 的 GORM 多列 Scan 陷阱已显式给出两种写法说明。✅

**3. Type consistency：** `buildPayload(provider models.AIProvider, req ChatRequest) map[string]any` 在 Task1 定义并被测试；`ArticleContext string` 在 Task3 定义，Task4 引用 `t.ArticleContext` 一致；`buildArticleContextForTag(tagID uint, start, end time.Time) string` 签名在 Task3 定义和调用一致。✅

**4. 跨包依赖安全：** Task3 明确「不 import tagmanagement，在 topicgraph 内重写 pickArticleSummary」，避免循环依赖。✅

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-27-qwythos-thinking-toggle-report-grounding.md`.

采用 subagent-driven-development 执行（用户指定）：每个 Task 派 implementer 子进程实现 + 自审 + 提交，然后派 spec-reviewer + code-quality-reviewer 两阶段审查；我（主控）负责进度/方向把控、处理 BLOCKED、Task 间衔接，全部 Task 完成后派最终 code reviewer + 跑门禁（§11 归档门禁）。
