package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
)

// setupRecFixture 构造推荐所需最小数据：全局桶偏好向量 + 3 条路由（ok/broken/ok）+ 各自向量。
// 所有向量同向（distance≈0），靠 status 验证粗筛排除规则。
func setupRecFixture(t *testing.T, db *gorm.DB) (r1, r2, r3 uint) {
	t.Helper()
	vec := floatsToPgVector(padVec([]float64{1, 0, 0}))
	routes := []models.RSSHubRoute{
		{Namespace: "nsa", Path: "/a", Name: "A", Example: "/nsa/a", UsableDirectly: true, Status: "ok", Parameters: "{}"},
		{Namespace: "nsb", Path: "/b", Name: "B", Example: "/nsb/b", UsableDirectly: true, Status: "broken", Parameters: "{}"},
		{Namespace: "nsc", Path: "/c", Name: "C", Example: "/nsc/c", UsableDirectly: true, Status: "ok", Parameters: "{}"},
	}
	for i := range routes {
		require.NoError(t, db.Create(&routes[i]).Error)
	}
	r1, r2, r3 = routes[0].ID, routes[1].ID, routes[2].ID
	for _, rid := range []uint{r1, r2, r3} {
		require.NoError(t, db.Create(&models.RouteEmbedding{
			RouteID: rid, EmbeddingVec: vec, Dimension: testutil.TestEmbeddingDim, Model: "test",
		}).Error)
	}
	require.NoError(t, db.Create(&models.PreferenceVector{
		BoardID: nil, Source: PreferenceSourceBehavior, EmbeddingVec: vec,
		Dimension: testutil.TestEmbeddingDim, Model: "test",
	}).Error)
	return
}

// TestRecommendationRefreshExcludesBroken：粗筛排除 status=broken 的路由（D4/D5）。
func TestRecommendationRefreshExcludesBroken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	r1, r2, r3 := setupRecFixture(t, db)
	svc := NewRecommendationService(db, nil, nil)

	_, err := svc.RefreshRecommendations(context.Background())
	require.NoError(t, err)

	var recs []models.FeedRecommendation
	require.NoError(t, db.Where("status = ?", "pending").Find(&recs).Error)
	routeIDs := make(map[uint]bool)
	for _, r := range recs {
		routeIDs[r.RouteID] = true
	}
	require.True(t, routeIDs[r1], "ok 路由 r1 应被推荐")
	require.True(t, routeIDs[r3], "ok 路由 r3 应被推荐")
	require.False(t, routeIDs[r2], "broken 路由 r2 应被排除")
}

// TestRecommendationAcceptCreatesFeed：接受 usable_directly 推荐 → 创建 feed + 标 accepted。
func TestRecommendationAcceptCreatesFeed(t *testing.T) {
	db := testutil.SetupTestDB(t)
	r1, _, _ := setupRecFixture(t, db)
	svc := NewRecommendationService(db, nil, nil)

	_, err := svc.RefreshRecommendations(context.Background())
	require.NoError(t, err)

	var rec models.FeedRecommendation
	require.NoError(t, db.Where("route_id = ? AND status = ?", r1, "pending").First(&rec).Error)

	feed, err := svc.AcceptRecommendation(context.Background(), rec.ID, nil, nil)
	require.NoError(t, err)
	require.NotZero(t, feed.ID)
	require.Equal(t, DefaultRSSHubBaseURL+"/nsa/a", feed.URL)

	var after models.FeedRecommendation
	require.NoError(t, db.First(&after, rec.ID).Error)
	require.Equal(t, "accepted", after.Status)
	require.NotNil(t, after.AcceptedFeedID)
}

// TestRecommendationDismissCooldown：dismiss 后冷却期内同 hash 不再入库。
func TestRecommendationDismissCooldown(t *testing.T) {
	db := testutil.SetupTestDB(t)
	r1, _, _ := setupRecFixture(t, db)
	svc := NewRecommendationService(db, nil, nil)

	_, err := svc.RefreshRecommendations(context.Background())
	require.NoError(t, err)
	var rec models.FeedRecommendation
	require.NoError(t, db.Where("route_id = ?", r1).First(&rec).Error)

	require.NoError(t, svc.DismissRecommendation(context.Background(), rec.ID))

	// 再次刷新：r1 已 dismiss，冷却期内不再入库。
	s2, err := svc.RefreshRecommendations(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, s2.Inserted, "dismiss 冷却期内同 route+board 不应再入库")

	var cnt int64
	db.Model(&models.FeedRecommendation{}).Where("route_id = ?", r1).Count(&cnt)
	require.EqualValues(t, 1, cnt, "r1 仅一条记录（dismissed）")
}

// TestRecommendationDismissCooldownExpiredReuseRow：dismiss 冷却过期后重推，
// 复用既有 dismissed 行回到 pending，不因 recommendation_hash UNIQUE 冲突报错（H1）。
func TestRecommendationDismissCooldownExpiredReuseRow(t *testing.T) {
	db := testutil.SetupTestDB(t)
	r1, _, _ := setupRecFixture(t, db)
	svc := NewRecommendationService(db, nil, nil)

	_, err := svc.RefreshRecommendations(context.Background())
	require.NoError(t, err)
	var rec models.FeedRecommendation
	require.NoError(t, db.Where("route_id = ?", r1).First(&rec).Error)
	require.NoError(t, svc.DismissRecommendation(context.Background(), rec.ID))

	// 模拟冷却过期：dismissed_at 改到 31 天前（超过默认 30 天冷却）。
	require.NoError(t, db.Model(&models.FeedRecommendation{}).Where("id = ?", rec.ID).
		Update("dismissed_at", time.Now().AddDate(0, 0, -31)).Error)

	// 再刷新：r1 应复用 dismissed 行回到 pending，不报 UNIQUE 错。
	s2, err := svc.RefreshRecommendations(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, s2.Inserted, "冷却过期后 r1 应复用入库")

	var after models.FeedRecommendation
	require.NoError(t, db.Where("route_id = ?", r1).First(&after).Error)
	require.Equal(t, "pending", after.Status, "应回到 pending")
	require.Nil(t, after.DismissedAt, "dismissed_at 应清空")

	var cnt int64
	db.Model(&models.FeedRecommendation{}).Where("route_id = ?", r1).Count(&cnt)
	require.EqualValues(t, 1, cnt, "r1 仍仅一条记录（复用非新建）")
}

// TestRecommendationCardIncludesRouteStatus：卡片暴露路由 status，供前端标「未验证/broken」（spec feed-discovery）。
func TestRecommendationCardIncludesRouteStatus(t *testing.T) {
	db := testutil.SetupTestDB(t)
	_, _, _ = setupRecFixture(t, db)
	svc := NewRecommendationService(db, nil, nil)

	_, err := svc.RefreshRecommendations(context.Background())
	require.NoError(t, err)

	cards, err := svc.GetRecommendations(context.Background(), "pending")
	require.NoError(t, err)
	require.NotEmpty(t, cards)
	for _, c := range cards {
		require.Equal(t, "ok", c.RouteStatus, "fixture 路由 status=ok 应透传到卡片")
	}
}

// TestBuildFeedURLEscapesParams：requires_parameters 路由，用户填的参数值含特殊字符要 path-escape（M2）。
func TestBuildFeedURLEscapesParams(t *testing.T) {
	r := &models.RSSHubRoute{
		Namespace: "bilibili", Path: "/user/dynamic/:uid",
		RequiresParameters: true, UsableDirectly: false,
	}
	u := buildFeedURL(r, map[string]string{"uid": "a b/c"}, DefaultRSSHubBaseURL)
	require.NotContains(t, u, " ", "空格应转义")
	require.Contains(t, u, "a%20b", "空格转 %20")
	require.Contains(t, u, "%2F", "/ 应转义防 path 注入")
}
