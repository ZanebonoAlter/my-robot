#!/usr/bin/env bash
# check-standards.sh 参数范围冒烟：以独立最小 fixture 覆盖 F 段，绝不读取主工作区 active changes。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="$REPO_ROOT/scripts/check-standards.sh"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/check-standards-smoke.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT

pass=0
fail=0
ok() { printf '  [OK] %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf '  [FAIL] %s\n' "$1" >&2; fail=$((fail + 1)); }
run_capture() {
	local label="$1" expected="$2"
	shift 2
	local output rc
	set +e
	output="$(cd "$TMP" && "$@" 2>&1)"
	rc=$?
	set -e
	if [ "$expected" = "zero" ] && [ "$rc" -eq 0 ]; then
		ok "$label"
	elif [ "$expected" = "nonzero" ] && [ "$rc" -ne 0 ]; then
		ok "$label"
	else
		bad "$label（exit $rc，输出：$output）"
	fi
	RUN_OUTPUT="$output"
}

mkdir -p "$TMP/scripts" "$TMP/openspec/changes" \
	"$TMP/docs/reference/standard/frontend" \
	"$TMP/docs/reference/standard/backend" \
	"$TMP/docs/reference/standard/shared" \
	"$TMP/docs/reference/flow" "$TMP/docs/reference/architecture" \
	"$TMP/backend-go/internal/models" "$TMP/front/app/assets/css"
cp "$SOURCE" "$TMP/scripts/check-standards.sh"
chmod +x "$TMP/scripts/check-standards.sh"

# A/D/G 所需最小文档结构；AGENTS.md 聚合文件名，避免 fixture 受真实仓库状态影响。
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
	docs/reference/architecture/map.md \
	front/eslint.config.js backend-go/.golangci.yml; do
	mkdir -p "$TMP/$(dirname "$f")"
	: >"$TMP/$f"
done
for flow in "$TMP"/docs/reference/flow/*.md; do
	[ "$(basename "$flow")" = "README.md" ] && continue
	printf '%s\n' \
		'## 需求说明' '## 链路设计' '## 业务约束与不变量' '## 代码入口' '## 变更溯源' \
		>"$flow"
done
printf '%s\n' \
	'--raw-fixture: 0;' '--color-fixture: 0;' '[data-theme="editorial"] {}' '[data-theme="dark"] {}' \
	>"$TMP/front/app/assets/css/main.css"
printf '%s\n' 'theming.md code-style.md lint.md testing.md package-layout.md commit-pr.md' >"$TMP/AGENTS.md"
for domain in admin dataenrichment reader tagmanagement topicgraph; do
	mkdir -p "$TMP/backend-go/internal/$domain/handler"
done
for model in ai_models.go topic_graph.go semantic_label.go; do
	: >"$TMP/backend-go/internal/models/$model"
done

# F 段专用可控替身：以 change basename 决定 verify 成败。
cat >"$TMP/scripts/doc-impact.sh" <<'EOF'
#!/usr/bin/env bash
set -eu
[ "${1:-}" = "verify" ] || exit 2
case "$(basename "${2:-}")" in
  *-fail) echo "fixture doc-impact failure: $(basename "$2")" >&2; exit 1 ;;
  *) exit 0 ;;
esac
EOF
chmod +x "$TMP/scripts/doc-impact.sh"
for name in target-pass unrelated-fail target-fail; do
	mkdir -p "$TMP/openspec/changes/$name"
	printf '%s\n' '<!-- doc-impact: none(fixture) -->' >"$TMP/openspec/changes/$name/tasks.md"
done

# 无参必须仍遍历全部 active change，因此 unrelated-fail 令 F 段失败。
run_capture '无参：全仓 F 段仍会报告无关 change 失败' nonzero bash scripts/check-standards.sh
[[ "$RUN_OUTPUT" == *'doc-impact 失败 unrelated-fail'* ]] || bad '无参输出包含 unrelated-fail 的 F 段失败'

# 带参数只选目标 F 段，不应被 unrelated-fail 阻断。
run_capture '--change：目标通过时忽略无关 change F 段失败' zero bash scripts/check-standards.sh --change target-pass
[[ "$RUN_OUTPUT" == *'doc-impact 通过 target-pass'* && "$RUN_OUTPUT" != *unrelated-fail* ]] \
	&& ok '--change 输出仅包含目标 F 段结果' \
	|| bad '--change F 段未严格限定到目标'

run_capture '--change：目标自身 doc-impact 失败仍失败' nonzero bash scripts/check-standards.sh --change target-fail
[[ "$RUN_OUTPUT" == *'doc-impact 失败 target-fail'* ]] \
	&& ok '目标自身失败原因可见' \
	|| bad '目标自身失败原因缺失'

for invocation in \
	'--change not-exist' \
	'--change' \
	'--unknown' \
	'--change nested/name'; do
	# shellcheck disable=SC2086 # fixture invocation 固定，故意覆盖缺参/非法参。
	run_capture "非法参数：$invocation 明确失败" nonzero bash scripts/check-standards.sh $invocation
	[[ "$RUN_OUTPUT" == *'错误'* || "$RUN_OUTPUT" == *'不存在'* ]] \
		&& ok "非法参数：$invocation 输出中文错误" \
		|| bad "非法参数：$invocation 未输出中文错误"
done

printf '\ncheck-standards smoke：通过 %d / 失败 %d\n' "$pass" "$fail"
[ "$fail" -eq 0 ] && printf 'SMOKE OK\n'
[ "$fail" -eq 0 ]
