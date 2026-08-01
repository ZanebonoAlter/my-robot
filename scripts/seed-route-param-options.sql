-- feed-param-options 字典 seed：人工录可选值（source='manual'）
-- 数据来源：RSSHub 官方源码 description 表格（GitHub raw；docs.rsshub.app 国内不可达的绕行）
--
-- 执行时机：重启后端（AutoMigrate 确保 route_param_options 表存在）后跑：
--   cat scripts/seed-route-param-options.sql | docker exec -i syntopica-postgres psql -U postgres -d syntopica
-- 幂等：ON CONFLICT DO NOTHING，可重复执行。

-- qbitai /category/:category
INSERT INTO route_param_options (route_id, param_name, value, label, source, created_at, updated_at)
SELECT r.id, 'category', v.value, v.label, 'manual', now(), now()
FROM rsshub_routes r
CROSS JOIN (VALUES
  ('资讯', '资讯'),
  ('ebandeng', '数码'),
  ('auto', '智能车'),
  ('zhiku', '智库'),
  ('huodong', '活动')
) AS v(value, label)
WHERE r.namespace = 'qbitai' AND r.path = '/category/:category'
ON CONFLICT (route_id, param_name, value) DO NOTHING;

-- tencent /pvp/newsindex/:type
INSERT INTO route_param_options (route_id, param_name, value, label, source, created_at, updated_at)
SELECT r.id, 'type', v.value, v.label, 'manual', now(), now()
FROM rsshub_routes r
CROSS JOIN (VALUES
  ('all', '全部'),
  ('rm', '热门'),
  ('xw', '新闻'),
  ('gg', '公告'),
  ('hd', '活动'),
  ('ss', '赛事'),
  ('yh', '优化')
) AS v(value, label)
WHERE r.namespace = 'tencent' AND r.path = '/pvp/newsindex/:type'
ON CONFLICT (route_id, param_name, value) DO NOTHING;

-- ithome /ranking/:type
INSERT INTO route_param_options (route_id, param_name, value, label, source, created_at, updated_at)
SELECT r.id, 'type', v.value, v.label, 'manual', now(), now()
FROM rsshub_routes r
CROSS JOIN (VALUES
  ('24h', '24小时阅读榜'),
  ('7days', '7天最热'),
  ('monthly', '月榜')
) AS v(value, label)
WHERE r.namespace = 'ithome' AND r.path = '/ranking/:type'
ON CONFLICT (route_id, param_name, value) DO NOTHING;

-- ithome /tw/feeds/:category
INSERT INTO route_param_options (route_id, param_name, value, label, source, created_at, updated_at)
SELECT r.id, 'category', v.value, v.label, 'manual', now(), now()
FROM rsshub_routes r
CROSS JOIN (VALUES
  ('news', '新聞'),
  ('big-data', 'AI'),
  ('cloud', 'Cloud'),
  ('devops', 'DevOps'),
  ('security', '資安')
) AS v(value, label)
WHERE r.namespace = 'ithome' AND r.path = '/tw/feeds/:category'
ON CONFLICT (route_id, param_name, value) DO NOTHING;

-- 36kr /:category/:subCategory?/:keyword?
INSERT INTO route_param_options (route_id, param_name, value, label, source, created_at, updated_at)
SELECT r.id, 'category', v.value, v.label, 'manual', now(), now()
FROM rsshub_routes r
CROSS JOIN (VALUES
  ('news', '最新资讯频道'),
  ('newsflashes', '快讯'),
  ('recommend', '推荐资讯'),
  ('life', '生活'),
  ('estate', '房产'),
  ('workplace', '职场')
) AS v(value, label)
WHERE r.namespace = '36kr' AND r.path = '/:category/:subCategory?/:keyword?'
ON CONFLICT (route_id, param_name, value) DO NOTHING;
