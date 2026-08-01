"""PoC 主入口:跑中东地缘新闻的完整三角色分析,生成判定报告。

用法: uv run main.py
报告输出到 poc_report.md
"""

from __future__ import annotations

import json

from orchestrator import run_analysis
from test_news import MIDDLE_EAST_NEWS, TECH_NEWS


def write_report(results: list[dict]) -> None:
    """生成 Markdown 判定报告。"""
    lines = ["# 数据增强 PoC 判定报告", "", f"生成时间: {results[0].get('timestamp', '')}", f"模型: Qwen3.5-9B (llama.cpp @ localhost:8080)", "", "---", ""]

    for r in results:
        lines += [
            f"## {r['label']}",
            "",
            f"**总耗时**: {r.get('total_sec', '?')}s (ETF加载 {r.get('etf_load_sec', '?')}s + 解读 {r.get('interpret_sec', '?')}s + 查询 {r.get('research_sec', '?')}s + 分析 {r.get('analysis_sec', '?')}s)",
            "",
            "### 新闻原文",
            "```",
            r["news"].strip(),
            "```",
            "",
            "### ① 解读员提炼的主题",
            "",
        ]
        topics = r.get("interpret", {}).get("topics", [])
        for t in topics:
            lines.append(f"- **{t.get('topic', '?')}**: {t.get('reason', '')}")
        lines.append("")

        # 角色② trace —— 判定报告的核心
        lines.append("### ② 查询员 agent loop trace(★核心验证对象)")
        lines.append("")
        for td in r.get("topics_data", []):
            lines += [
                f"#### 主题: {td['topic']}",
                f"- 循环次数: {td['loops']}  | 工具调用: {td['tool_call_count']} 次  | {'✅完成' if not td['error'] else '⚠ ' + td['error']}",
                "",
            ]
            for tc in td["tool_calls"]:
                args_str = json.dumps(tc["args"], ensure_ascii=False)
                lines += [
                    f"  **step{tc['step']}**: `{tc['tool']}({args_str})`",
                    f"    - thought: {tc['thought']}",
                    f"    - result: {tc['result_preview'][:150]}",
                    "",
                ]
            lines += [f"  **最终汇总**: {td.get('data', '(空)')[:200]}", ""]

        # 角色③ 结论
        analysis = r.get("analysis", {})
        lines.append("### ③ 分析员结论")
        lines.append("")
        for s in analysis.get("affected_sectors", []):
            lines.append(f"- **{s.get('sector', '?')}** [{s.get('direction', '?')}/{s.get('confidence', '?')}]")
            lines.append(f"  - {s.get('reasoning', '')}")
            if s.get("evidence_etf"):
                lines.append(f"  - 数据支撑: {s.get('evidence_etf', '')}")
            lines.append("")
        lines.append(f"> **总体**: {analysis.get('overall', '')}")
        lines += ["", "---", ""]

    # 判定小结
    lines += ["## 判定小结", ""]
    all_topics = []
    all_calls = 0
    all_loops = 0
    errors = 0
    for r in results:
        for td in r.get("topics_data", []):
            all_topics.append(td)
            all_calls += td["tool_call_count"]
            all_loops += td["loops"]
            if td["error"]:
                errors += 1
    lines += [
        f"- 共 {len(all_topics)} 个主题,总工具调用 {all_calls} 次,总循环 {all_loops} 次",
        f"- 出错主题数: {errors}/{len(all_topics)}",
        f"- 平均每主题循环: {all_loops/len(all_topics):.1f} 次" if all_topics else "",
        "",
        "**关键观察(人工填写)**:",
        "- [ ] 链式行为:查询员是否会先查目录(list_etf)再查详情(get_quote)?",
        "- [ ] 换词能力:命中少时,会主动换更宽泛的产业词重查吗?",
        "- [ ] 幻觉率:查的 ETF 代码真实存在吗?",
        "- [ ] 终止控制:拿到数据会及时停,还是无限查?",
        "- [ ] 领域适配:地缘新闻查的是石油/军工/黄金/航运,没瞎查半导体吧?",
        "- [ ] 最终质量:分析员的板块影响判断,有数据支撑、方向合理吗?",
    ]

    with open("poc_report.md", "w", encoding="utf-8") as f:
        f.write("\n".join(lines))
    print("\n报告已写入 poc_report.md")


def main() -> None:
    # 先跑中东地缘(主测试)
    results = []
    r1 = run_analysis(MIDDLE_EAST_NEWS, label="中东地缘冲突")
    results.append(r1)
    # 再跑科技类(对照,看领域适配)
    r2 = run_analysis(TECH_NEWS, label="国产光刻机突破")
    results.append(r2)

    write_report(results)


if __name__ == "__main__":
    main()
