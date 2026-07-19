#!/usr/bin/env bash
# check-standards.sh — L1 代码规范结构验收（归档前随《开发执行规范》§11.4 运行）
#
# 纯静态校验，不触发编译。WSL bash 可跑。检查维度：
#   A. 文档完整性（standard/ flow/ architecture/map.md 关键文件存在）
#   B. 后端结构（golangci 配置、domain 白名单、三层包结构）
#   C. 前端结构（ESLint 配置、Token 三层、双主题）
#   D. 防孤立引用（每个 standard/*.md 被至少一处 AGENTS.md / README 引用）
#   E. flow 变更溯源链接（archive change 被某 flow 文档「变更溯源」表引用，归档后校验）
#
# 用法： bash scripts/check-standards.sh
# 退出码：0 全过；1 有失败。

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

PASS=0
FAIL=0

ok() {
	echo "  [OK]   $1"
	PASS=$((PASS + 1))
}
fail() {
	echo "  [FAIL] $1"
	FAIL=$((FAIL + 1))
}

check_file() { [ -f "$1" ] && ok "存在 $1" || fail "缺失 $1"; }

echo "== A. 文档完整性 =="
# standard/ 权威层
for f in \
	docs/reference/standard/README.md \
	docs/reference/standard/frontend/theming.md \
	docs/reference/standard/frontend/code-style.md \
	docs/reference/standard/frontend/lint.md \
	docs/reference/standard/frontend/testing.md \
	docs/reference/standard/backend/package-layout.md \
	docs/reference/standard/backend/code-style.md \
	docs/reference/standard/backend/lint.md \
	docs/reference/standard/backend/testing.md \
	docs/reference/standard/shared/commit-pr.md \
	docs/reference/flow/README.md \
	docs/reference/flow/reading.md \
	docs/reference/flow/content-enrichment.md \
	docs/reference/flow/ai-summary.md \
	docs/reference/flow/daily-report.md \
	docs/reference/flow/topic-graph.md \
	docs/reference/flow/semantic-board.md \
	docs/reference/flow/scheduler.md \
	docs/reference/flow/data-enrichment.md \
	docs/reference/architecture/map.md; do
	check_file "$f"
done

# flow 文档五段式结构校验（docs-harness-consolidation 引入）
for f in docs/reference/flow/*.md; do
	[ -f "$f" ] || continue
	[ "$(basename "$f")" = "README.md" ] && continue
	missing=""
	for sec in 需求说明 链路设计 业务约束与不变量 代码入口 变更溯源; do
		grep -q "^## $sec" "$f" || missing="$missing $sec"
	done
	if [ -z "$missing" ]; then
		ok "$(basename "$f") 五段式齐全"
	else
		fail "$(basename "$f") 缺五段式:$missing"
	fi
done

echo ""
echo "== B. 后端结构 =="
# golangci 配置
check_file backend-go/.golangci.yml

# domain 白名单（权威：standard/backend/package-layout.md）
WHITELIST="admin dataenrichment reader tagmanagement topicgraph"

# 1) 每个"业务 domain 签名"（同时有 routes.go + handler/）必须在白名单内
for d in backend-go/internal/*/; do
	name="$(basename "$d")"
	if [ -f "$d/routes.go" ] && [ -d "$d/handler" ]; then
		# 是 domain 签名
		case " $WHITELIST " in
		*" $name "*) ok "domain $name 在白名单内且有三层结构" ;;
		*) fail "domain $name 有 routes.go+handler/ 但不在白名单（先登记到 standard/backend/package-layout.md）" ;;
		esac
	fi
done

# 2) 白名单内每个 domain 都真实存在且有 handler/
for name in $WHITELIST; do
	d="backend-go/internal/$name"
	if [ ! -d "$d" ]; then
		fail "白名单 domain $name 目录不存在"
		continue
	fi
	[ -d "$d/handler" ] && ok "$name/handler/ 存在" || fail "$name 缺 handler/ 子包"
done

echo ""
echo "== C. 前端结构 =="
check_file front/eslint.config.js

CSS="front/app/assets/css/main.css"
check_file "$CSS"
if [ -f "$CSS" ]; then
	grep -q -- "--raw-" "$CSS" && ok "Token Layer1 (Primitive --raw-*)" || fail "缺 Primitive token (--raw-*)"
	grep -q -- "--color-" "$CSS" && ok "Token Layer2 (Semantic --color-*)" || fail "缺 Semantic token (--color-*)"
	grep -q "data-theme" "$CSS" && ok "主题切换机制 (data-theme)" || fail "缺 data-theme 主题切换"
	if grep -q "editorial" "$CSS" && grep -q "dark" "$CSS"; then
		ok "双主题定义 (editorial + dark)"
	else
		fail "缺双主题 (editorial + dark)"
	fi
fi

echo ""
echo "== D. 防孤立引用 =="
# 每个 standard/*.md（README 除外）必须被至少一处 AGENTS.md / README 引用
REF_FILES="front/AGENTS.md backend-go/AGENTS.md AGENTS.md docs/reference/standard/README.md docs/reference/开发执行规范.md"
for f in docs/reference/standard/frontend/*.md docs/reference/standard/backend/*.md docs/reference/standard/shared/*.md; do
	base="$(basename "$f")"
	if grep -rq "$base" $REF_FILES 2>/dev/null; then
		ok "被引用 $base"
	else
		fail "孤立文档 $base 未被任何 AGENTS.md / README 引用"
	fi
done

echo ""
echo "== E. flow 变更溯源链接（归档后校验，见《开发执行规范》§12.2）=="
# 新流程生效日（2026-06-29）之后的 archive change 必须被 flow 文档的变更溯源表引用；
# 历史存量免校验，避免一次性爆 FAIL。
CUTOFF="2026-06-29"
FLOW_DIR="docs/reference/flow"
if [ -d "openspec/changes/archive" ]; then
	for d in openspec/changes/archive/*/; do
		[ -d "$d" ] || continue
		name="$(basename "$d")"
		arch_date="${name:0:10}"
		# 只校验生效日及之后的 archive（YYYY-MM-DD 字典序比较 = 日期比较）
		if [[ "$arch_date" < "$CUTOFF" ]]; then
			continue
		fi
		# §12.2 豁免：tasks.md「文档」节声明「无 flow 影响」的 change 不触及任何业务 flow，免溯源校验
		if [ -f "$d/tasks.md" ] && grep -q "无 flow 影响" "$d/tasks.md" 2>/dev/null; then
			ok "豁免溯源 $name（tasks.md 声明无 flow 影响）"
			continue
		fi
		if grep -rq "$name" "$FLOW_DIR"/*.md 2>/dev/null; then
			ok "已溯源 $name"
		else
			fail "未溯源 $name（在 docs/reference/flow/*.md 变更溯源表补一行链接回该 archive）"
		fi
	done
fi

echo ""
echo "== F. doc-impact 声明对账（见《开发执行规范》§11.4）=="
# 只校验已声明 doc-impact 的 change（本 capability 首次引入于 docs-harness-consolidation，
# 此前的 active change 无声明属正常，跳过；新 change 声明了才对账）。
if [ -f scripts/doc-impact.sh ]; then
	for d in openspec/changes/*/; do
		[ -d "$d" ] || continue
		name="$(basename "$d")"
		[ "$name" = "archive" ] && continue
		# 过渡期：未声明 doc-impact 的旧 change 跳过
		if [ -f "$d/tasks.md" ] && grep -q '<!-- doc-impact:' "$d/tasks.md" 2>/dev/null; then
			if bash scripts/doc-impact.sh verify "$d" >/dev/null 2>&1; then
				ok "doc-impact 通过 $name"
			else
				fail "doc-impact 失败 $name（跑 bash scripts/doc-impact.sh verify $d 看详情）"
			fi
		else
			ok "跳过 $name（未声明 doc-impact）"
		fi
	done
else
	fail "scripts/doc-impact.sh 不存在，F 段无法运行"
fi

echo ""
echo "== G. 导航层文档死链检查 =="
# 只查导航层：docs/README.md、docs/reference/*.md（一级）、flow/README.md、architecture/map.md
check_md_links() {
	local file="$1" dir link target
	dir="$(dirname "$file")"
	while IFS= read -r link; do
		case "$link" in http* | mailto* | \#*) continue ;; esac
		target="${link%%#*}"
		[ -e "$dir/$target" ] || fail "死链 $file → $link"
	done < <(grep -oE '\]\([^)]+\.md\)' "$file" 2>/dev/null | sed -E 's/^\]\(//; s/\)$//')
}
NAV_FILES="docs/README.md docs/reference/flow/README.md docs/reference/architecture/map.md"
for f in docs/reference/*.md; do [ -f "$f" ] && NAV_FILES="$NAV_FILES $f"; done
for f in $NAV_FILES; do
	[ -f "$f" ] || continue
	check_md_links "$f"
done

echo ""
echo "=============================="
echo "  通过 $PASS / 失败 $FAIL"
echo "=============================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
