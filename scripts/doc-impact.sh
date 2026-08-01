#!/usr/bin/env bash
# doc-impact.sh — 选项式文档影响门禁（docs-harness-consolidation 引入）
#
# 把「文档要更新哪些」从回忆题变成选择题。四个子命令：
#   menu                      打印 8 个固定文档域选项菜单
#   suggest [--base <ref>]    按 git diff 启发式预勾选 + 命中理由 + 声明注释模板
#   context [--base <ref>]    按 git diff 命中 domain dump 相关 flow「业务约束与不变量」节（apply 改代码前必读）
#   verify <change-dir> [--base <ref>]  归档前对账（声明↔git diff + 反向启发式 + 文件存在性）
#     tasks.md 可加 <!-- doc-impact-excuse: domain=理由; ... --> 豁免"疑似遗漏"误报
#
# 详见 openspec/changes/docs-harness-consolidation/design.md §1。
# WSL bash 可跑。退出码：0 过；1 有失败（仅 verify 会非零）。

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# backend domain 白名单（与 check-standards.sh B 段一致，权威：standard/backend/package-layout.md）
DOMAIN_WHITELIST="admin dataenrichment reader tagmanagement topicgraph"

# ---------------------------------------------------------------------------
# 公共：取本 change 的改动文件集合（已跟踪改动 + 未跟踪新文件）
#   $1 = base ref（默认 HEAD）。工作区/暂存/未跟踪都算「本次改动」。
# ---------------------------------------------------------------------------
changed_files() {
	local base="${1:-HEAD}"
	# 已跟踪：git diff（含未 stage 的工作区改动 + 已 stage）
	# core.quotepath=false 让中文路径原样输出（否则 git 八进制转义，verify 比对失败）
	# core.checkStat=minimal：WSL DrvFS(/mnt/*) 上完整 stat 极慢（29s→2s），
	# 对启发式门禁而言 mtime+size 粒度足够
	git -c core.checkStat=minimal -c core.quotepath=false diff --name-only "$base" 2>/dev/null
	git -c core.quotepath=false diff --cached --name-only 2>/dev/null
	# 未跟踪新文件
	git -c core.quotepath=false ls-files --others --exclude-standard 2>/dev/null
}

# ---------------------------------------------------------------------------
# 启发式：某文档域是否被改动文件集合命中
#   $1 = 域 key；$2 = 预先算好的改动文件列表（换行分隔）。命中则 echo 命中的
#   文件路径（可能多个），否则空。
#   ⚠ 调用方必须预计算文件列表传入——曾在函数内每次重跑 changed_files，
#   verify/suggest 的 7 域循环导致 7+ 次全量 git 扫描（DrvFS 上每次 ~30s）。
# ---------------------------------------------------------------------------
heuristic_hit() {
	local domain="$1" files="$2"
	case "$domain" in
	flow)
		echo "$files" | grep -E 'backend-go/internal/(admin|reader|tagmanagement|topicgraph|dataenrichment)/service/' ||
			echo "$files" | grep -E '^front/app/features/'
		;;
	api)
		echo "$files" | grep -E 'backend-go/internal/[^/]+/handler/' ||
			echo "$files" | grep -E 'backend-go/internal/app/router\.go'
		;;
	database)
		echo "$files" | grep -E 'backend-go/internal/models/' ||
			{ echo "$files" | grep -E '^backend-go/.*\.go$' | xargs -r grep -lE 'AutoMigrate|CREATE TABLE|ALTER TABLE' 2>/dev/null; }
		;;
	architecture)
		echo "$files" | grep -E 'backend-go/internal/app/' ||
			echo "$files" | grep -E 'backend-go/internal/platform/tracing/'
		;;
	standard)
		echo "$files" | grep -E '(^|/)\.golangci\.yml|eslint\.config\.js' ||
			echo "$files" | grep -E '^docs/reference/standard/'
		;;
	configuration)
		echo "$files" | grep -E 'config.*\.ya?ml' ||
			echo "$files" | grep -E 'backend-go/internal/platform/config/'
		;;
	deployment)
		echo "$files" | grep -E 'Dockerfile|docker-compose' ||
			echo "$files" | grep -E '(^|/)(init\.ps1|init\.sh)$'
		;;
	none) ;;
	esac
}

# ---------------------------------------------------------------------------
# menu：打印 8 域选项
# ---------------------------------------------------------------------------
cmd_menu() {
	cat <<'EOF'
文档影响域（固定 8 选项，apply 启动时按 git diff 启发式预勾选）：
  flow          业务链路文档 docs/reference/flow/
  api           API 参考 docs/reference/api/
  database      数据库 docs/reference/database/
  architecture  架构 docs/reference/architecture/
  standard      代码规约 docs/reference/standard/
  configuration 配置 docs/reference/configuration.md
  deployment    部署 docs/reference/deployment.md
  none          纯内部重构（声明须附理由）
EOF
}

# ---------------------------------------------------------------------------
# suggest：预勾选 + 命中理由 + 声明模板
# ---------------------------------------------------------------------------
cmd_suggest() {
	local base="HEAD"
	[ "${1:-}" = "--base" ] && base="${2:-HEAD}"
	echo "文档域预勾选（--base $base）："
	local declared="" files
	files="$(changed_files "$base" | sort -u)"
	for domain in flow api database architecture standard configuration deployment; do
		local hit
		hit="$(heuristic_hit "$domain" "$files")"
		if [ -n "$hit" ]; then
			local first
			first="$(echo "$hit" | head -1)"
			printf '  [x] %-14s — 命中: %s\n' "$domain" "$first"
			declared="$declared $domain"
		else
			printf '  [ ] %-14s\n' "$domain"
		fi
	done
	if [ -z "${declared// /}" ]; then
		echo '  [x] none         — 未命中任何启发式（若是纯内部重构，声明 none 并附理由）'
		echo
		echo '请确认后写入 <change>/tasks.md「文档」节第一行：'
		echo '  <!-- doc-impact: none(纯内部重构，无对外行为变化) -->'
	else
		echo
		echo '请确认后写入 <change>/tasks.md「文档」节第一行：'
		echo "  <!-- doc-impact:${declared} -->"
	fi
}

# ---------------------------------------------------------------------------
# context：按命中 domain dump 相关 flow 业务约束节（apply 前置必读）
# ---------------------------------------------------------------------------
cmd_context() {
	local base="HEAD"
	[ "${1:-}" = "--base" ] && base="${2:-HEAD}"

	local files
	files="$(changed_files "$base" | sort -u)"

	# ========== 业务规范（what）：按命中 domain dump flow 业务约束节 ==========
	# 从改动文件提取命中的 backend domain
	local hit_domains=""
	for d in $DOMAIN_WHITELIST; do
		if echo "$files" | grep -qE "backend-go/internal/$d/"; then
			hit_domains="$hit_domains $d"
		fi
	done

	local what_out=""
	if [ -z "${hit_domains// /}" ]; then
		what_out="未识别到相关业务约束 flow；如改动涉及业务逻辑，请主动查阅 docs/reference/flow/"
	else
		what_out="命中 domain:${hit_domains}"
		what_out="$what_out"$'\n'"（按 flow 文档「代码入口」节关联匹配，dump 其「业务约束与不变量」节）"
		local dumped=0
		for flow in docs/reference/flow/*.md; do
			[ -f "$flow" ] || continue
			[ "$(basename "$flow")" = "README.md" ] && continue
			# 提取「代码入口」节，看是否提及任一命中 domain
			local entry_section
			entry_section="$(awk '/^## 代码入口/{f=1;next} /^## /{f=0} f' "$flow" 2>/dev/null)"
			local matched=0
			for d in $hit_domains; do
				if echo "$entry_section" | grep -qE "internal/$d/|/$d/|$d"; then
					matched=1
					break
				fi
			done
			[ "$matched" -eq 0 ] && continue

			what_out="$what_out"$'\n\n'"──────── $(basename "$flow") ────────"
			local constraint_section
			constraint_section="$(awk '/^## 业务约束与不变量/{f=1;next} /^## /{f=0} f' "$flow" 2>/dev/null)"
			if [ -n "$constraint_section" ]; then
				what_out="$what_out"$'\n'"$constraint_section"
			else
				what_out="$what_out"$'\n'"（⚠ 该 flow 尚未补齐「业务约束与不变量」节）"
			fi
			dumped=$((dumped + 1))
		done
		if [ "$dumped" -eq 0 ]; then
			what_out="$what_out"$'\n\n'"命中 domain 但无 flow 文档「代码入口」节关联（flow 五段式可能未补齐）。如改动涉及业务逻辑，请主动查阅 docs/reference/flow/ 并补全「代码入口」节。"
		fi
	fi

	# ========== 执行规范（how）：按 doc-impact-applies 匹配 standard Requirements ==========
	# 遍历 standard 文档，按文档头 doc-impact-applies 标签（逗号分隔多 token，路径前缀匹配）
	# 命中改动代码路径的文档 → dump 其 ## Requirements 节；MUST 级条目前缀 🛑。
	local how_out="" how_hit=0
	while IFS= read -r std; do
		[ -f "$std" ] || continue
		# 解析文档头 doc-impact-applies 标签（取该行，去前缀与可能的 -->）
		local applies
		applies="$(grep 'doc-impact-applies:' "$std" 2>/dev/null | head -1 | sed 's/.*doc-impact-applies://; s/-->.*//; s/^[[:space:]]*//; s/[[:space:]]*$//')"
		[ -z "$applies" ] && continue   # 无 applies 标签的文档跳过（过渡期未 spec 化）

		# 逗号分隔多 token，每个做路径前缀匹配（token 去尾斜杠；file==token 或以 token/ 开头）
		local tok matched=0 applies_spaced
		applies_spaced="$(printf '%s' "$applies" | tr ',' ' ' | tr -s ' ')"
		for tok in $applies_spaced; do
			tok="${tok%/}"
			[ -z "$tok" ] && continue
			local tok_re
			tok_re="$(printf '%s' "$tok" | sed 's/[.[\*^$()+?{|\\]/\\&/g')"
			if echo "$files" | grep -qE "^${tok_re}($|/)"; then
				matched=1
				break
			fi
		done
		[ "$matched" -eq 0 ] && continue

		# dump ## Requirements 节（无此节的非 spec 化文档静默跳过，过渡期行为）
		local req_section
		req_section="$(awk '/^## Requirements/{f=1;next} /^## /{f=0} f' "$std" 2>/dev/null)"
		[ -z "$req_section" ] && continue

		# MUST 级别行前缀 🛑，SHOULD 前缀 [SHOULD]
		req_section="$(printf '%s' "$req_section" | awk '
/^\*\*级别\*\*:[[:space:]]*MUST/ { print "🛑 " $0; next }
/^\*\*级别\*\*:[[:space:]]*SHOULD/ { print "[SHOULD] " $0; next }
{ print }
')"

		how_out="$how_out"$'\n\n'"──────── $(basename "$std") ────────"
		how_out="$how_out"$'\n'"$req_section"
		how_hit=$((how_hit + 1))
	done < <(find docs/reference/standard -name '*.md' ! -name 'README.md' 2>/dev/null | sort)

	if [ "$how_hit" -eq 0 ]; then
		how_out="未识别到相关执行规范"
	fi

	# ========== 输出双段 ==========
	echo "──── 业务规范（理解任务：what）────"
	printf '%s\n' "$what_out"
	echo
	echo "──── 执行规范（写代码：how）────"
	printf '%s\n' "$how_out"
	exit 0
}

# ---------------------------------------------------------------------------
# verify：归档前对账（5 规则，任一 FAIL 则退出码 1）
#   两层解析：(a) 域注释 <!-- doc-impact: ... -->；(b) 文档节 checkbox 文件路径
# ---------------------------------------------------------------------------
cmd_verify() {
	local change_dir="${1:-}"
	local base="HEAD"
	shift || true
	while [ $# -gt 0 ]; do
		case "$1" in
		--base)
			base="$2"
			shift 2
			;;
		*) shift ;;
		esac
	done

	if [ -z "$change_dir" ] || [ ! -f "$change_dir/tasks.md" ]; then
		echo "用法: doc-impact.sh verify <change-dir> [--base <ref>]" >&2
		echo "FAIL: change 目录无效或无 tasks.md" >&2
		exit 1
	fi

	local tasks="$change_dir/tasks.md"
	local v_fail=0
	local fails=""

	add_fail() {
		fails="${fails}  - $1\n"
		v_fail=$((v_fail + 1))
	}

	# --- 解析 (a) 域注释 ---
	local decl_line
	decl_line="$(grep -m1 '<!-- doc-impact:' "$tasks" 2>/dev/null || true)"
	if [ -z "$decl_line" ]; then
		add_fail "未声明 doc-impact（apply 启动时跑 suggest 补声明）"
		# 无声明则后续规则无意义，直接出结果
		printf '%b' "$fails" >&2
		echo "verify: $change_dir — $v_fail FAIL" >&2
		exit 1
	fi
	# 提取域列表（去掉 none(理由) 的括号内容）
	local decl_raw
	decl_raw="$(echo "$decl_line" | sed -E 's/.*<!-- doc-impact: *//; s/ *-->.*//; s/none\([^)]*\)/none/')"
	local declared_domains=" $decl_raw "

	# --- 解析豁免声明 <!-- doc-impact-excuse: domain=理由; ... --> ---
	# 豁免的域不触发"疑似遗漏"（巬发式过严/多 change 脏工作树的误报兑底）
	local excuse_line excused_domains
	excuse_line="$(grep -m1 '<!-- doc-impact-excuse:' "$tasks" 2>/dev/null || true)"
	excused_domains="$(echo "$excuse_line" | sed -E 's/.*<!-- doc-impact-excuse: *//; s/ *-->.*//' | tr ';' '\n' | sed -E 's/=.*//' | tr -d ' ' | tr '\n' ' ')"
	excused_domains=" $excused_domains "

	# --- 解析 (b) 文档节 checkbox 文件路径 ---
	# 取「## N. 文档」节内 - [ ] 后的路径
	local declared_files
	declared_files="$(awk '/^## [0-9]+\. 文档/{f=1;next} /^## [0-9]+\./{f=0} f' "$tasks" |
		grep -E '^[[:space:]]*- \[[ x]\]' |
		grep -oE 'docs/[^ "`)]+\.md' | sort -u)"

	local changed
	changed="$(changed_files "$base" | sort -u)"

	# --- 规则 4：声明 none 但启发式命中 ---
	if echo "$declared_domains" | grep -qw none; then
		for domain in flow api database architecture standard configuration deployment; do
			if [ -n "$(heuristic_hit "$domain" "$changed")" ]; then
				add_fail "声明 none 但启发式命中 $domain"
			fi
		done
	else
		# --- 规则 3：反向启发式命中未声明域 ---
		for domain in flow api database architecture standard configuration deployment; do
			if [ -n "$(heuristic_hit "$domain" "$changed")" ] && ! echo "$declared_domains" | grep -qw "$domain" && ! echo "$excused_domains" | grep -qw "$domain"; then
				add_fail "疑似遗漏: 改了 ${domain} 相关代码未声明 $domain"
			fi
		done
	fi

	# --- 规则 2 & 5：checkbox 声明的文件 ---
	if [ -n "$declared_files" ]; then
		while IFS= read -r f; do
			[ -z "$f" ] && continue
			# 规则 5：路径不存在
			if [ ! -e "$f" ]; then
				add_fail "声明的文档不存在: $f"
				continue
			fi
			# 规则 2：不在 changed 集合，且未在 git 历史提交
			# （事后补归档场景：change 改动已 commit 进主线，工作树 diff 为空；
			#   只要文件曾被提交即视为「已更新」，避免 base=HEAD 对已提交改动的误报）
			if ! echo "$changed" | grep -qxF "$f"; then
				if ! git log --oneline -- "$f" 2>/dev/null | grep -q .; then
					add_fail "声明了未更新: $f"
				fi
			fi
		done <<<"$declared_files"
	fi

	# --- 输出 ---
	if [ "$v_fail" -gt 0 ]; then
		printf '%b' "$fails" >&2
		echo "verify: $change_dir — $v_fail FAIL" >&2
		exit 1
	fi
	echo "verify: $change_dir — 通过（声明:${declared_domains} 文件:$(printf '%s\n' "$declared_files" | grep -c .) 个）"
	exit 0
}

# ---------------------------------------------------------------------------
# 入口
# ---------------------------------------------------------------------------
sub="${1:-}"
case "$sub" in
menu)
	shift
	cmd_menu "$@"
	;;
suggest)
	shift
	cmd_suggest "$@"
	;;
context)
	shift
	cmd_context "$@"
	;;
verify)
	shift
	cmd_verify "$@"
	;;
"" | -h | --help | help)
	sed -n '2,12p' "$0"
	echo
	echo "用法: bash scripts/doc-impact.sh <menu|suggest|context|verify> [...]"
	;;
*)
	echo "未知子命令: $sub" >&2
	exit 2
	;;
esac
