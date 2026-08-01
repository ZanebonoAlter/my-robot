-- 升级建议扩展/发现算法验证（证据采集，只读）
-- V1: 泳道签名 vs composition 签名 vs 真值（内容实际流向）
-- V2: 建议调用频率
-- V3: aux 标签近重复对规模

\echo '===== V0 基线数量 ====='
SELECT
 (SELECT COUNT(*) FROM semantic_labels WHERE label_type='auxiliary' AND status='active') AS aux_total,
 (SELECT COUNT(*) FROM semantic_labels s WHERE s.label_type='auxiliary' AND s.status='active' AND s.ref_count>=5
   AND NOT EXISTS (SELECT 1 FROM board_composition bc WHERE bc.auxiliary_label_id=s.id)) AS discovery_cands,
 (SELECT COUNT(*) FROM daily_report_sections s
   JOIN board_daily_reports r ON r.id=s.report_id
   JOIN board_persistent_topics t ON t.id=s.persistent_topic_id
  WHERE t.status='active' AND s.embedding IS NOT NULL
    AND r.period_date >= CURRENT_DATE - INTERVAL '30 days') AS active_lane_sections_30d;

\echo ''
\echo '===== V1a 真值：候选 aux 标签的内容今天实际流向哪个版块（top board 及占比）====='
WITH cand AS (
  SELECT s.id, s.label FROM semantic_labels s
  WHERE s.label_type='auxiliary' AND s.status='active' AND s.ref_count>=5
    AND NOT EXISTS (SELECT 1 FROM board_composition bc WHERE bc.auxiliary_label_id=s.id)
),
flow AS (
  SELECT tsl.semantic_label_id AS cand_id, bl.semantic_board_id AS board_id,
         COUNT(DISTINCT bl.topic_tag_id) AS tags
  FROM topic_tag_semantic_labels tsl
  JOIN topic_tag_board_labels bl ON bl.topic_tag_id = tsl.topic_tag_id
  JOIN cand c ON c.id = tsl.semantic_label_id
  GROUP BY 1,2
),
ranked AS (
  SELECT cand_id, board_id, tags,
         ROW_NUMBER() OVER (PARTITION BY cand_id ORDER BY tags DESC, board_id) rn,
         SUM(tags) OVER (PARTITION BY cand_id) tot
  FROM flow
)
SELECT c.label AS candidate, b.label AS truth_top_board, r.tags AS tags_in_top, r.tot AS tags_total,
       ROUND(r.tags::numeric/NULLIF(r.tot,0),2) AS share
FROM ranked r JOIN cand c ON c.id=r.cand_id JOIN semantic_labels b ON b.id=r.board_id
WHERE r.rn=1 ORDER BY r.tags DESC LIMIT 25;

\echo ''
\echo '===== V1b 泳道签名：候选 → 各版块 active 泳道 section 最小距离（最优/次优/margin）====='
WITH cand AS (
  SELECT s.id, s.label, s.embedding::vector AS emb FROM semantic_labels s
  WHERE s.label_type='auxiliary' AND s.status='active' AND s.ref_count>=5
    AND NOT EXISTS (SELECT 1 FROM board_composition bc WHERE bc.auxiliary_label_id=s.id)
    AND s.embedding IS NOT NULL
),
lane AS (
  SELECT r.semantic_board_id AS board_id, s.embedding::vector AS emb
  FROM daily_report_sections s
  JOIN board_daily_reports r ON r.id = s.report_id
  JOIN board_persistent_topics t ON t.id = s.persistent_topic_id
  WHERE t.status='active' AND s.embedding IS NOT NULL
    AND r.period_date >= CURRENT_DATE - INTERVAL '30 days'
),
dist AS (
  SELECT c.id AS cand_id, l.board_id, MIN(c.emb <=> l.emb) AS min_dist
  FROM cand c CROSS JOIN lane l GROUP BY c.id, l.board_id
),
ranked AS (
  SELECT cand_id, board_id, min_dist,
         ROW_NUMBER() OVER (PARTITION BY cand_id ORDER BY min_dist) rn
  FROM dist
)
SELECT c.label AS candidate, b1.label AS lane_best_board, r1.min_dist::numeric(5,4) AS best_dist,
       b2.label AS lane_second, r2.min_dist::numeric(5,4) AS second_dist,
       (r2.min_dist-r1.min_dist)::numeric(5,4) AS margin
FROM ranked r1
JOIN ranked r2 ON r2.cand_id=r1.cand_id AND r2.rn=2
JOIN (SELECT DISTINCT id,label FROM cand) c ON c.id=r1.cand_id
JOIN semantic_labels b1 ON b1.id=r1.board_id
JOIN semantic_labels b2 ON b2.id=r2.board_id
WHERE r1.rn=1 ORDER BY r1.min_dist LIMIT 25;

\echo ''
\echo '===== V1c composition 签名：同一公式，对照组 ====='
WITH cand AS (
  SELECT s.id, s.label, s.embedding::vector AS emb FROM semantic_labels s
  WHERE s.label_type='auxiliary' AND s.status='active' AND s.ref_count>=5
    AND NOT EXISTS (SELECT 1 FROM board_composition bc WHERE bc.auxiliary_label_id=s.id)
    AND s.embedding IS NOT NULL
),
comp AS (
  SELECT bc.board_id, a.embedding::vector AS emb
  FROM board_composition bc
  JOIN semantic_labels a ON a.id=bc.auxiliary_label_id
  WHERE a.status='active' AND a.embedding IS NOT NULL
),
dist AS (
  SELECT c.id AS cand_id, p.board_id, MIN(c.emb <=> p.emb) AS min_dist
  FROM cand c CROSS JOIN comp p GROUP BY c.id, p.board_id
),
ranked AS (
  SELECT cand_id, board_id, min_dist,
         ROW_NUMBER() OVER (PARTITION BY cand_id ORDER BY min_dist) rn
  FROM dist
)
SELECT c.label AS candidate, b1.label AS comp_best_board, r1.min_dist::numeric(5,4) AS best_dist,
       (r2.min_dist-r1.min_dist)::numeric(5,4) AS margin
FROM ranked r1
JOIN ranked r2 ON r2.cand_id=r1.cand_id AND r2.rn=2
JOIN (SELECT DISTINCT id,label FROM cand) c ON c.id=r1.cand_id
JOIN semantic_labels b1 ON b1.id=r1.board_id
WHERE r1.rn=1 ORDER BY r1.min_dist LIMIT 25;

\echo ''
\echo '===== V1d 签名命中率裁决：泳道/comp 最优版块 vs 真值 top 版块 ====='
WITH cand AS (
  SELECT s.id, s.label, s.embedding::vector AS emb FROM semantic_labels s
  WHERE s.label_type='auxiliary' AND s.status='active' AND s.ref_count>=5
    AND NOT EXISTS (SELECT 1 FROM board_composition bc WHERE bc.auxiliary_label_id=s.id)
    AND s.embedding IS NOT NULL
),
truth AS (
  SELECT DISTINCT ON (tsl.semantic_label_id) tsl.semantic_label_id AS cand_id, bl.semantic_board_id AS board_id,
         COUNT(DISTINCT bl.topic_tag_id) AS tags
  FROM topic_tag_semantic_labels tsl
  JOIN topic_tag_board_labels bl ON bl.topic_tag_id=tsl.topic_tag_id
  JOIN cand c ON c.id=tsl.semantic_label_id
  GROUP BY 1,2 ORDER BY 1,3 DESC
),
lane AS (
  SELECT r.semantic_board_id AS board_id, s.embedding::vector AS emb
  FROM daily_report_sections s
  JOIN board_daily_reports r ON r.id=s.report_id
  JOIN board_persistent_topics t ON t.id=s.persistent_topic_id
  WHERE t.status='active' AND s.embedding IS NOT NULL
    AND r.period_date >= CURRENT_DATE - INTERVAL '30 days'
),
comp AS (
  SELECT bc.board_id, a.embedding::vector AS emb
  FROM board_composition bc JOIN semantic_labels a ON a.id=bc.auxiliary_label_id
  WHERE a.status='active' AND a.embedding IS NOT NULL
),
lane_best AS (
  SELECT DISTINCT ON (c.id) c.id AS cand_id, l.board_id, MIN(c.emb <=> l.emb) AS d
  FROM cand c CROSS JOIN lane l GROUP BY c.id, l.board_id ORDER BY c.id, d
),
comp_best AS (
  SELECT DISTINCT ON (c.id) c.id AS cand_id, p.board_id, MIN(c.emb <=> p.emb) AS d
  FROM cand c CROSS JOIN comp p GROUP BY c.id, p.board_id ORDER BY c.id, d
)
SELECT COUNT(*) AS candidates_with_truth,
       COUNT(*) FILTER (WHERE lb.board_id = t.board_id) AS lane_hit,
       COUNT(*) FILTER (WHERE cb.board_id = t.board_id) AS comp_hit,
       COUNT(*) FILTER (WHERE lb.board_id = t.board_id AND cb.board_id <> t.board_id) AS lane_only_hit,
       COUNT(*) FILTER (WHERE cb.board_id = t.board_id AND lb.board_id <> t.board_id) AS comp_only_hit
FROM truth t
JOIN lane_best lb ON lb.cand_id=t.cand_id
JOIN comp_best cb ON cb.cand_id=t.cand_id;

\echo ''
\echo '===== V2 升级建议 LLM 调用频率（近30天）====='
SELECT operation, COUNT(*) AS calls, MAX(created_at)::date AS last_call
FROM ai_call_logs
WHERE operation LIKE '%upgrade%' OR operation LIKE '%board%suggest%'
GROUP BY operation ORDER BY calls DESC;

\echo ''
\echo '===== V3a aux 标签近重复对（cos dist < 0.05，取最像的 20 对）====='
WITH aux AS (
  SELECT id, label, embedding::vector AS emb FROM semantic_labels
  WHERE label_type='auxiliary' AND status='active' AND embedding IS NOT NULL
)
SELECT a.label AS label_a, b.label AS label_b, (a.emb <=> b.emb)::numeric(5,4) AS dist
FROM aux a JOIN aux b ON a.id < b.id
WHERE a.emb <=> b.emb < 0.05
ORDER BY dist LIMIT 20;

\echo ''
\echo '===== V3b 近重复对总数（分桶）====='
WITH aux AS (
  SELECT id, embedding::vector AS emb FROM semantic_labels
  WHERE label_type='auxiliary' AND status='active' AND embedding IS NOT NULL
),
pairs AS (
  SELECT a.emb <=> b.emb AS d FROM aux a JOIN aux b ON a.id < b.id
)
SELECT COUNT(*) FILTER (WHERE d < 0.05) AS dup_lt_0_05,
       COUNT(*) FILTER (WHERE d >= 0.05 AND d < 0.10) AS near_0_05_0_10,
       COUNT(*) AS total_pairs
FROM pairs;
