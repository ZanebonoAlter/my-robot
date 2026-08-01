package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
)

// marshalNamespace 构造 /api/namespace 的 mock 响应（ns → {routes: {path: detail}}）。
func marshalNamespace(t *testing.T, routes map[string]map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := make(map[string]json.RawMessage)
	for ns, rs := range routes {
		body := map[string]any{"routes": rs}
		b, err := json.Marshal(body)
		require.NoError(t, err)
		out[ns] = b
	}
	return out
}

// TestFlattenNamespace 验证嵌套 dict 展平（D2 解析）。
func TestFlattenNamespace(t *testing.T) {
	raw := marshalNamespace(t, map[string]map[string]any{
		"36kr": {
			"/newsflashes": map[string]any{
				"path": "/36kr/newsflashes", "name": "快讯", "url": "36kr.com",
				"example": "/36kr/newsflashes", "description": "36氪快讯",
				"parameters": map[string]any{},
			},
		},
		"bilibili": {
			"/user/dynamic/:uid": map[string]any{
				"path": "/bilibili/user/dynamic/:uid", "name": "用户动态",
				"example": "/bilibili/user/dynamic/1", "description": "B 站用户动态",
				"parameters": map[string]any{"uid": "用户 UID"},
			},
		},
	})
	recs := flattenNamespace(raw)
	require.Len(t, recs, 2)
	byPath := make(map[string]routeRecord, len(recs))
	for _, r := range recs {
		byPath[r.Path] = r
	}
	require.Contains(t, byPath, "/36kr/newsflashes")
	require.Contains(t, byPath, "/bilibili/user/dynamic/:uid")
	require.Equal(t, "快讯", byPath["/36kr/newsflashes"].Name)
	require.Equal(t, `{"uid":"用户 UID"}`, byPath["/bilibili/user/dynamic/:uid"].ParametersJSON())
}

// TestCatalogSyncAllInsertAndParamMark：首次同步入库 + 参数标记（testcontainer）。
func TestCatalogSyncAllInsertAndParamMark(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := NewCatalogSyncService(db, "")
	svc.fetch = func(ctx context.Context) (map[string]json.RawMessage, error) {
		return marshalNamespace(t, map[string]map[string]any{
			"36kr": {
				"/newsflashes": map[string]any{
					"path": "/36kr/newsflashes", "name": "快讯",
					"description": "36氪快讯", "parameters": map[string]any{},
				},
			},
			"bilibili": {
				"/user/dynamic/:uid": map[string]any{
					"path": "/bilibili/user/dynamic/:uid", "name": "用户动态",
					"description": "B 站动态", "parameters": map[string]any{"uid": "用户 UID"},
				},
			},
		}), nil
	}

	summary, err := svc.SyncAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, summary.Total)
	require.Equal(t, 2, summary.Inserted)

	var usable, requires models.RSSHubRoute
	require.NoError(t, db.Where("namespace = ? AND path = ?", "36kr", "/36kr/newsflashes").First(&usable).Error)
	require.True(t, usable.UsableDirectly, "零参数路由 usable_directly")
	require.False(t, usable.RequiresParameters)

	require.NoError(t, db.Where("namespace = ? AND path = ?", "bilibili", "/bilibili/user/dynamic/:uid").First(&requires).Error)
	require.True(t, requires.RequiresParameters, "必填参数路由 requires_parameters")
	require.False(t, requires.UsableDirectly)
}

// TestCatalogSyncAllIdempotent：二次同步内容不变 → 不产生变更。
func TestCatalogSyncAllIdempotent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	fetch := func(ctx context.Context) (map[string]json.RawMessage, error) {
		return marshalNamespace(t, map[string]map[string]any{
			"36kr": {"/newsflashes": map[string]any{"path": "/36kr/newsflashes", "name": "快讯", "description": "d"}},
		}), nil
	}
	svc := NewCatalogSyncService(db, "")
	svc.fetch = fetch

	_, err := svc.SyncAll(context.Background())
	require.NoError(t, err)
	s2, err := svc.SyncAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, s2.Inserted, "幂等：无新增")
	require.Equal(t, 0, s2.Updated, "幂等：无变更")
}

// TestCatalogSyncAllMarksGone：目录消失的路由标 gone（不物理删除）。
func TestCatalogSyncAllMarksGone(t *testing.T) {
	db := testutil.SetupTestDB(t)
	full := func(ctx context.Context) (map[string]json.RawMessage, error) {
		return marshalNamespace(t, map[string]map[string]any{
			"36kr": {"/newsflashes": map[string]any{"path": "/36kr/newsflashes", "name": "快讯", "description": "d"}},
		}), nil
	}
	empty := func(ctx context.Context) (map[string]json.RawMessage, error) {
		return marshalNamespace(t, map[string]map[string]any{}), nil
	}
	svc := NewCatalogSyncService(db, "")
	svc.fetch = full
	_, err := svc.SyncAll(context.Background())
	require.NoError(t, err)

	svc.fetch = empty
	s2, err := svc.SyncAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, s2.Gone, "消失路由应标 gone")

	var gone models.RSSHubRoute
	require.NoError(t, db.Where("namespace = ? AND path = ?", "36kr", "/36kr/newsflashes").First(&gone).Error)
	require.Equal(t, "gone", gone.Status, "路由保留但状态 gone（不物理删除）")
}

// TestCatalogSyncAllUnreachable：fetch 失败 → 不修改既有目录，仅记日志。
func TestCatalogSyncAllUnreachable(t *testing.T) {
	db := testutil.SetupTestDB(t)
	// 预置一行
	require.NoError(t, db.Create(&models.RSSHubRoute{Namespace: "x", Path: "/p", Name: "n", Parameters: "{}", ContentHash: "h", Status: "unknown"}).Error)
	svc := NewCatalogSyncService(db, "")
	svc.fetch = func(ctx context.Context) (map[string]json.RawMessage, error) {
		return nil, gorm.ErrInvalidDB // 模拟不可达
	}
	summary, err := svc.SyncAll(context.Background())
	require.NoError(t, err, "不可达不应返回错误（保留既有目录）")
	require.Equal(t, 0, summary.Total)
	var cnt int64
	db.Model(&models.RSSHubRoute{}).Count(&cnt)
	require.EqualValues(t, 1, cnt, "既有目录保持不变")
}
