package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/logging"
)

// ── 订阅源推荐（design D5/D6，feed-discovery spec）──
//
// 两段式：pgvector 粗筛（route_embeddings <=> preference_vectors）→ LLM 精排。
// recommendation_hash = route_id+board_id（不含 source）：qa 与 manual_refresh 共享幂等池与 dismiss 冷却池。
// 排除规则（D5/B）：broken / 已 accepted 的 route / dismissed 冷却期内的 route / usable_directly 且 feeds.url 已存在。

// RecommendationSource 枚举 feed_recommendations.source。
const (
	RecommendationSourceManualRefresh = "manual_refresh"
	RecommendationSourceQA            = "qa"
)

// RecommendationService 实现推荐生成、状态机与问答。
type RecommendationService struct {
	db      *gorm.DB
	router  *airouter.Router // 精排 LLM + 问答 embedding；nil 时精排直出粗筛、问答不可用
	prefSvc *PreferenceProfileService
}

// NewRecommendationService 构造。prefSvc 为 nil 时内部按需创建（用于问答种子写入）。
func NewRecommendationService(db *gorm.DB, router *airouter.Router, prefSvc *PreferenceProfileService) *RecommendationService {
	if prefSvc == nil {
		prefSvc = NewPreferenceProfileService(db)
	}
	return &RecommendationService{db: db, router: router, prefSvc: prefSvc}
}

// RefreshSummary 描述一轮推荐刷新的产出。
type RefreshSummary struct {
	Candidates      int `json:"candidates"`
	Inserted        int `json:"inserted"`
	Skipped         int `json:"skipped"`          // hash 已 pending
	CooldownBlocked int `json:"cooldown_blocked"` // dismiss 冷却期内
}

// RefreshRecommendations 手动刷新（粗筛 + 精排 + 幂等落库），source=manual_refresh。
func (s *RecommendationService) RefreshRecommendations(ctx context.Context) (*RefreshSummary, error) {
	return s.generateAndPersist(ctx, RecommendationSourceManualRefresh, nil)
}

// candidateRow 粗筛候选。
type candidateRow struct {
	RouteID            uint
	Namespace          string
	Path               string
	Name               string
	Description        string
	Example            string
	UsableDirectly     bool
	RequiresParameters bool
	Parameters         string
	BoardID            *uint
	Distance           float64
}

// generateAndPersist 粗筛 → 精排 → 幂等落库。qaVec 非 nil 时用问答向量替代 preference_vectors 粗筛。
func (s *RecommendationService) generateAndPersist(ctx context.Context, source string, qaVec []float64) (*RefreshSummary, error) {
	topN := RecommendationTopNDefault
	cooldownDays := DismissCooldownDaysDefault

	candidates, err := s.coarseFilter(ctx, topN, cooldownDays, qaVec)
	if err != nil {
		return nil, fmt.Errorf("coarse filter: %w", err)
	}
	// usable_directly 额外按 feeds.url 去重（D5/B）。
	candidates = s.filterByFeedsURL(ctx, candidates)

	summary := &RefreshSummary{Candidates: len(candidates)}
	if len(candidates) == 0 {
		return summary, nil
	}

	// 精排：router 非 nil 走 LLM；否则直出（score = 1-distance，reason 空）。
	ranked := s.rerank(ctx, candidates)

	now := time.Now()
	for _, rc := range ranked {
		hash := ComputeRecommendationHash(rc.RouteID, rc.BoardID)
		// dismiss 冷却（跨 source，按 hash）。
		blocked, err := s.countDismissedInCooldown(ctx, hash, cooldownDays)
		if err != nil {
			return summary, err
		}
		if blocked > 0 {
			summary.CooldownBlocked++
			continue
		}
		inserted, err := s.insertPending(ctx, models.FeedRecommendation{
			RouteID:            rc.RouteID,
			BoardID:            rc.BoardID,
			Source:             source,
			Score:              rc.Score,
			LLMReason:          rc.Reason,
			Status:             "pending",
			RecommendationHash: hash,
		}, now)
		if err != nil {
			return summary, err
		}
		if inserted {
			summary.Inserted++
		} else {
			summary.Skipped++
		}
	}
	logging.Infof("recommendation refresh(%s): candidates=%d inserted=%d skipped=%d cooldown=%d",
		source, summary.Candidates, summary.Inserted, summary.Skipped, summary.CooldownBlocked)
	return summary, nil
}

// coarseFilter pgvector 粗筛：每版块偏好向量 top-N，排除 broken/accepted/dismissed-cooldown。
// qaVec 非 nil 时仅用该向量（问答即时推荐，单全局桶）。
func (s *RecommendationService) coarseFilter(ctx context.Context, topN, cooldownDays int, qaVec []float64) ([]candidateRow, error) {
	if qaVec != nil {
		return s.coarseFilterByVector(ctx, qaVec, topN, cooldownDays)
	}
	// 取所有 preference_vectors（behavior+seed），每条做一次 top-N。
	type pvRow struct {
		BoardID      *uint
		EmbeddingVec string
		Dimension    int
	}
	var pvs []pvRow
	if err := s.db.WithContext(ctx).
		Raw(`SELECT board_id, embedding AS embedding_vec, dimension FROM preference_vectors WHERE source IN ('behavior','seed')`).
		Scan(&pvs).Error; err != nil {
		return nil, err
	}
	var all []candidateRow
	seen := make(map[uint]struct{})
	for _, pv := range pvs {
		vec, err := parsePgVector(pv.EmbeddingVec)
		if err != nil || len(vec) == 0 {
			continue
		}
		rows, err := s.coarseFilterByVectorBoard(ctx, vec, pv.Dimension, pv.BoardID, topN, cooldownDays)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if _, ok := seen[r.RouteID]; ok {
				continue
			}
			seen[r.RouteID] = struct{}{}
			all = append(all, r)
		}
	}
	return all, nil
}

// coarseFilterByVector 对单个问答向量粗筛（board_id=NULL，全局桶）。
func (s *RecommendationService) coarseFilterByVector(ctx context.Context, vec []float64, topN, cooldownDays int) ([]candidateRow, error) {
	return s.coarseFilterByVectorBoard(ctx, vec, len(vec), nil, topN, cooldownDays)
}

// coarseFilterByVectorBoard 对单向量 + 指定 board 粗筛（pgvector <=>）。
func (s *RecommendationService) coarseFilterByVectorBoard(
	ctx context.Context, vec []float64, dim int, boardID *uint, topN, cooldownDays int,
) ([]candidateRow, error) {
	vecStr := floatsToPgVector(vec)
	q := `
		SELECT r.id AS route_id, r.namespace, r.path, r.name, r.description, r.example,
		       r.usable_directly, r.requires_parameters, r.parameters,
		       (e.embedding <=> ?::vector) AS distance
		FROM route_embeddings e
		JOIN rsshub_routes r ON r.id = e.route_id
		WHERE e.dimension = ?
		  AND r.status NOT IN ('broken','gone')
		  AND r.id NOT IN (SELECT route_id FROM feed_recommendations WHERE status = 'accepted')
		  AND r.id NOT IN (
		    SELECT route_id FROM feed_recommendations
		    WHERE status = 'dismissed' AND dismissed_at IS NOT NULL
		      AND dismissed_at > NOW() - make_interval(days => ?)
		  )
		ORDER BY e.embedding <=> ?::vector
		LIMIT ?`
	type row struct {
		RouteID            uint
		Namespace          string
		Path               string
		Name               string
		Description        string
		Example            string
		UsableDirectly     bool
		RequiresParameters bool
		Parameters         string
		Distance           float64
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(q, vecStr, dim, cooldownDays, vecStr, topN).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]candidateRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, candidateRow{
			RouteID: r.RouteID, Namespace: r.Namespace, Path: r.Path, Name: r.Name,
			Description: r.Description, Example: r.Example,
			UsableDirectly: r.UsableDirectly, RequiresParameters: r.RequiresParameters,
			Parameters: r.Parameters, BoardID: boardID, Distance: r.Distance,
		})
	}
	return out, nil
}

// filterByFeedsURL 对 usable_directly 候选按 feeds.url 去重（D5/B）。
func (s *RecommendationService) filterByFeedsURL(ctx context.Context, cs []candidateRow) []candidateRow {
	if len(cs) == 0 {
		return cs
	}
	// 预取所有 feeds.url 集合。
	var urls []string
	s.db.WithContext(ctx).Model(&models.Feed{}).Pluck("url", &urls)
	urlSet := make(map[string]struct{}, len(urls))
	for _, u := range urls {
		urlSet[u] = struct{}{}
	}
	out := cs[:0]
	baseURL := resolveRSSHubBaseURL(s.db)
	for _, c := range cs {
		if c.UsableDirectly {
			expected := baseURL + c.Example
			if _, ok := urlSet[expected]; ok {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// rankedCandidate 精排后结果。
type rankedCandidate struct {
	candidateRow
	Score  float64
	Reason string
}

// rerank LLM 精排；router 为 nil 时直出粗筛（score=1-distance）。
func (s *RecommendationService) rerank(ctx context.Context, cs []candidateRow) []rankedCandidate {
	out := make([]rankedCandidate, 0, len(cs))
	if s.router == nil {
		for _, c := range cs {
			out = append(out, rankedCandidate{candidateRow: c, Score: 1 - c.Distance})
		}
		return out
	}
	// LLM 精排：构造候选摘要，请求保留子集 + 理由（失败则降级直出）。
	prompt := buildRerankPrompt(cs)
	chatResp, err := s.router.Chat(ctx, airouter.ChatRequest{
		Capability: airouter.CapabilityFeedDiscovery,
		Messages:   []airouter.Message{{Role: "user", Content: prompt}},
		Operation:  "discovery.recommendation_rerank",
	})
	if err != nil || chatResp == nil || chatResp.Content == "" {
		for _, c := range cs {
			out = append(out, rankedCandidate{candidateRow: c, Score: 1 - c.Distance})
		}
		return out
	}
	reasons := parseRerankResponse(chatResp.Content, cs)
	for _, c := range cs {
		reason := reasons[c.RouteID]
		out = append(out, rankedCandidate{candidateRow: c, Score: 1 - c.Distance, Reason: reason})
	}
	return out
}

// insertPending 幂等插入 pending 行：复用同 hash 现有行（dismissed 冷却过期重推→回 pending），
// 同 hash 已 pending → 不重复；无现有行 → 新建。避免 recommendation_hash UNIQUE 冲突（H1）。
func (s *RecommendationService) insertPending(ctx context.Context, rec models.FeedRecommendation, now time.Time) (bool, error) {
	var existing models.FeedRecommendation
	err := s.db.WithContext(ctx).Where("recommendation_hash = ?", rec.RecommendationHash).First(&existing).Error
	if err == nil {
		if existing.Status == "pending" {
			return false, nil // 同 hash 已 pending：幂等不重复
		}
		// dismissed（冷却过期重推）或异常残留 → 复用行回 pending，清 dismiss/accept 痕迹。
		existing.Source = rec.Source
		existing.Score = rec.Score
		existing.LLMReason = rec.LLMReason
		existing.BoardID = rec.BoardID
		existing.Status = "pending"
		existing.DismissedAt = nil
		existing.AcceptedFeedID = nil
		existing.UpdatedAt = now
		return true, s.db.WithContext(ctx).Save(&existing).Error
	}
	if !isNotFound(err) {
		return false, err
	}
	rec.CreatedAt = now
	rec.UpdatedAt = now
	return true, s.db.WithContext(ctx).Create(&rec).Error
}

// countDismissedInCooldown 统计 hash 在冷却期内的 dismiss 数（跨 source）。
func (s *RecommendationService) countDismissedInCooldown(ctx context.Context, hash string, days int) (int64, error) {
	var c int64
	err := s.db.WithContext(ctx).Model(&models.FeedRecommendation{}).
		Where("recommendation_hash = ? AND status = 'dismissed' AND dismissed_at IS NOT NULL AND dismissed_at > NOW() - make_interval(days => ?)",
			hash, days).
		Count(&c).Error
	return c, err
}

// RecommendationCard 是推荐卡片视图（含路由元数据）。
type RecommendationCard struct {
	models.FeedRecommendation
	RouteNamespace     string `json:"route_namespace"`
	RoutePath          string `json:"route_path"`
	RouteName          string `json:"route_name"`
	RouteExample       string `json:"route_example"`
	UsableDirectly     bool   `json:"usable_directly"`
	RequiresParameters bool   `json:"requires_parameters"`
	Parameters         string `json:"parameters"`
	RouteStatus        string `json:"route_status"`
	BoardLabel         string `json:"board_label"`
}

// GetRecommendations 返回推荐卡片列表（默认 pending）。
func (s *RecommendationService) GetRecommendations(ctx context.Context, status string) ([]RecommendationCard, error) {
	if status == "" {
		status = "pending"
	}
	var recs []models.FeedRecommendation
	err := s.db.WithContext(ctx).
		Preload("Route").Preload("Board").
		Where("status = ?", status).
		Order("created_at DESC").
		Find(&recs).Error
	if err != nil {
		return nil, err
	}
	cards := make([]RecommendationCard, 0, len(recs))
	for _, r := range recs {
		card := RecommendationCard{FeedRecommendation: r}
		if r.Route != nil {
			card.RouteNamespace = r.Route.Namespace
			card.RoutePath = r.Route.Path
			card.RouteName = r.Route.Name
			card.RouteExample = r.Route.Example
			card.UsableDirectly = r.Route.UsableDirectly
			card.RequiresParameters = r.Route.RequiresParameters
			card.Parameters = r.Route.Parameters
			card.RouteStatus = r.Route.Status
		}
		if r.Board != nil {
			card.BoardLabel = r.Board.Label
		}
		cards = append(cards, card)
	}
	return cards, nil
}

// AcceptRecommendation 接受推荐：usable_directly 直订；requires_parameters 填参拼 URL 订阅。
// 订阅成功后标记 accepted 并记录 feed_id。
func (s *RecommendationService) AcceptRecommendation(ctx context.Context, id uint, categoryID *uint, params map[string]string) (*models.Feed, error) {
	var rec models.FeedRecommendation
	if err := s.db.WithContext(ctx).Preload("Route").First(&rec, id).Error; err != nil {
		return nil, err
	}
	if rec.Status != "pending" {
		return nil, fmt.Errorf("recommendation %d not pending (status=%s)", id, rec.Status)
	}
	if rec.Route == nil {
		return nil, fmt.Errorf("recommendation %d has no route", id)
	}
	feedURL := buildFeedURL(rec.Route, params, resolveRSSHubBaseURL(s.db))
	if feedURL == "" {
		return nil, fmt.Errorf("cannot resolve feed url for route %s", rec.Route.Path)
	}
	// URL 已存在 → 复用既有 feed。
	var existing models.Feed
	if err := s.db.WithContext(ctx).Where("url = ?", feedURL).First(&existing).Error; err == nil {
		if err := s.markAccepted(ctx, &rec, existing.ID); err != nil {
			return nil, err
		}
		return &existing, nil
	}
	now := time.Now()
	feed := models.Feed{
		Title: firstNonEmpty(rec.Route.Name, "Untitled Feed"),
		URL:   feedURL, CategoryID: categoryID,
		Icon: "mdi:rss", IconSource: "fallback", Color: "#8b5cf6",
		MaxArticles: 100, RefreshInterval: 60, LastUpdated: &now,
	}
	if err := s.db.WithContext(ctx).Create(&feed).Error; err != nil {
		return nil, fmt.Errorf("create feed: %w", err)
	}
	if err := s.markAccepted(ctx, &rec, feed.ID); err != nil {
		return nil, err
	}
	return &feed, nil
}

// markAccepted 标记推荐 accepted 并关联 feed_id。
func (s *RecommendationService) markAccepted(ctx context.Context, rec *models.FeedRecommendation, feedID uint) error {
	now := time.Now()
	rec.Status = "accepted"
	rec.AcceptedFeedID = &feedID
	rec.UpdatedAt = now
	return s.db.WithContext(ctx).Save(rec).Error
}

// DismissRecommendation 拒绝推荐，进入冷却期。
func (s *RecommendationService) DismissRecommendation(ctx context.Context, id uint) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.FeedRecommendation{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]any{"status": "dismissed", "dismissed_at": now, "updated_at": now}).Error
}

// Ask 问答式即时推荐：embedding → 粗筛 → 精排 → 落库(source=qa) + 种子写入。
func (s *RecommendationService) Ask(ctx context.Context, question string) ([]RecommendationCard, error) {
	if s.router == nil {
		return nil, fmt.Errorf("ask requires airouter (embedding route not configured)")
	}
	result, err := s.router.Embed(ctx, airouter.EmbeddingRequest{
		Input: []string{question}, Operation: "discovery.ask",
	}, airouter.CapabilityEmbedding)
	if err != nil {
		return nil, fmt.Errorf("embed question: %w", err)
	}
	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("empty embedding for question")
	}
	qaVec := result.Embeddings[0]

	// 即时粗筛 + 精排（不入库，仅返回）。
	candidates, err := s.coarseFilterByVector(ctx, qaVec, RecommendationTopNDefault, DismissCooldownDaysDefault)
	if err != nil {
		return nil, err
	}
	candidates = s.filterByFeedsURL(ctx, candidates)
	ranked := s.rerank(ctx, candidates)

	// 落库 source=qa（幂等 + 冷却）。
	now := time.Now()
	for _, rc := range ranked {
		hash := ComputeRecommendationHash(rc.RouteID, nil)
		blocked, _ := s.countDismissedInCooldown(ctx, hash, DismissCooldownDaysDefault)
		if blocked > 0 {
			continue
		}
		_, _ = s.insertPending(ctx, models.FeedRecommendation{
			RouteID: rc.RouteID, BoardID: nil, Source: RecommendationSourceQA,
			Score: rc.Score, LLMReason: rc.Reason, Status: "pending",
			RecommendationHash: hash,
		}, now)
	}

	// 种子写入：问题向量匹配板块落 seed 行（D7）。
	boardVecs, _ := s.loadBoardVectors(ctx)
	_ = s.prefSvc.WriteSeed(ctx, qaVec, result.Dimensions, result.Model, boardVecs)

	// 返回即时卡片。
	return s.cardsFromRanked(ctx, ranked)
}

// loadBoardVectors 取各版块向量（SemanticLabel.Embedding）。
func (s *RecommendationService) loadBoardVectors(ctx context.Context) (map[uint][]float64, error) {
	type row struct {
		ID        uint
		Embedding *string
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(`SELECT id, embedding FROM semantic_labels WHERE embedding IS NOT NULL`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint][]float64)
	for _, r := range rows {
		if r.Embedding == nil {
			continue
		}
		v, err := parsePgVector(*r.Embedding)
		if err != nil || len(v) == 0 {
			continue
		}
		out[r.ID] = v
	}
	return out, nil
}

// cardsFromRanked 把精排结果转成卡片视图。
func (s *RecommendationService) cardsFromRanked(ctx context.Context, ranked []rankedCandidate) ([]RecommendationCard, error) {
	if len(ranked) == 0 {
		return []RecommendationCard{}, nil
	}
	ids := make([]uint, 0, len(ranked))
	for _, r := range ranked {
		ids = append(ids, r.RouteID)
	}
	var routes []models.RSSHubRoute
	s.db.WithContext(ctx).Where("id IN ?", ids).Find(&routes)
	byID := make(map[uint]models.RSSHubRoute, len(routes))
	for _, r := range routes {
		byID[r.ID] = r
	}
	cards := make([]RecommendationCard, 0, len(ranked))
	for _, r := range ranked {
		rt := byID[r.RouteID]
		cards = append(cards, RecommendationCard{
			RouteNamespace: rt.Namespace, RoutePath: rt.Path, RouteName: rt.Name,
			RouteExample: rt.Example, UsableDirectly: rt.UsableDirectly,
			RequiresParameters: rt.RequiresParameters, Parameters: rt.Parameters, RouteStatus: rt.Status,
			FeedRecommendation: models.FeedRecommendation{
				RouteID: r.RouteID, Source: RecommendationSourceQA, Score: r.Score,
				LLMReason: r.Reason, Status: "pending",
				RecommendationHash: ComputeRecommendationHash(r.RouteID, nil),
			},
		})
	}
	return cards, nil
}

// buildFeedURL 拼接受订阅的 feed URL。
// usable_directly：baseURL + example（或 namespace+path）；requires_parameters：用 params 填 path 参数。
func buildFeedURL(r *models.RSSHubRoute, params map[string]string, baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	if r.UsableDirectly {
		if r.Example != "" {
			return base + r.Example
		}
		return base + "/" + r.Namespace + r.Path
	}
	// requires_parameters：把 path 中的 :param 用 params 填充（未提供的可选参数跳过）。
	u := "/" + r.Namespace + r.Path
	for name, val := range params {
		u = strings.ReplaceAll(u, ":"+name, url.PathEscape(val))
	}
	// 去掉剩余可选参数段 :x? 与正则约束 {..}。
	u = stripOptionalParams(u)
	if !strings.Contains(u, ":") {
		return base + u
	}
	return "" // 仍有未填必填参数
}

// stripOptionalParams 去掉 :param? 与 {regex} 残留。
func stripOptionalParams(url string) string {
	// 去正则 {..}
	for strings.Contains(url, "{") {
		i := strings.Index(url, "{")
		j := strings.Index(url, "}")
		if j < i {
			break
		}
		url = url[:i] + url[j+1:]
	}
	// 去可选参数段 :xxx?（整段连同前导 /）
	out := []string{}
	for _, seg := range strings.Split(url, "/") {
		if strings.HasPrefix(seg, ":") && strings.HasSuffix(seg, "?") {
			continue
		}
		out = append(out, seg)
	}
	return strings.Join(out, "/")
}

// buildRerankPrompt 构造 LLM 精排 prompt。
func buildRerankPrompt(cs []candidateRow) string {
	var b strings.Builder
	b.WriteString("以下是候选 RSS 订阅源，请从中挑选最值得推荐的，并为每条写一句中文推荐理由（引用路由 name/description）。\n")
	b.WriteString("返回 JSON 数组，每项 {\"route_id\": 数字, \"reason\": \"理由\"}。不要返回未挑选的路由。\n\n")
	for _, c := range cs {
		fmt.Fprintf(&b, "- route_id=%d | %s/%s | %s | %s\n", c.RouteID, c.Namespace, c.Path, c.Name, truncate(c.Description, 100))
	}
	return b.String()
}

// parseRerankResponse 解析 LLM 精排响应 → map[routeID]reason。
func parseRerankResponse(content string, cs []candidateRow) map[uint]string {
	out := map[uint]string{}
	// 提取首个 JSON 数组。
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start < 0 || end <= start {
		return out
	}
	var items []struct {
		RouteID uint   `json:"route_id"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &items); err != nil {
		return out
	}
	for _, it := range items {
		if it.Reason != "" {
			out[it.RouteID] = it.Reason
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
