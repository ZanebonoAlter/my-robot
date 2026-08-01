"""演进版 PoC 主入口:消费持久话题的演进脉络,跑三角色分析。

用法: uv run python main_evolved.py
区别 vs main.py: 输入是 topic 的演进脉络(GetTopicLifeline 结构),不是孤立新闻。
"""

from __future__ import annotations

import json
import time
from datetime import datetime

from lifeline_mock import LITHOGRAPHY_LIFELINE, MIDDLE_EAST_LIFELINE, render_lifeline_for_agent
from roles_evolved import analyze_evolved_impact, interpret_lifeline, research_topic_evolved
from tools import _load_etf_spot


def run_for_topic(lifeline: dict, label: str = "") -> dict:
    """跑一个持久话题的完整演进分析,返回 trace。"""
    print(f"\n{'='*70}")
    print(f"开始演进分析 [{label}]")
    print(f"{'='*70}")

    lifeline_text = render_lifeline_for_agent(lifeline)
    result = {"label": label, "topic": lifeline["topic"]["label"], "timestamp": datetime.now().isoformat()}

    # 预热 ETF 缓存
    t0 = time.time()
    print("\n[预热] 加载全市场 ETF 行情...")
    _load_etf_spot()
    result["etf_load_sec"] = round(time.time() - t0, 1)

    # 角色① 解读员:从演进脉络提炼需查的主题
    print("\n[①解读员] 从演进脉络提炼需补数据的产业主题...")
    t1 = time.time()
    interp = interpret_lifeline(lifeline_text)
    result["interpret_sec"] = round(time.time() - t1, 1)
    topics = interp.get("topics", [])
    result["interpret"] = interp
    print(f"  提炼出 {len(topics)} 个主题:")
    for t in topics:
        print(f"    - {t.get('topic', '?')}: {t.get('reason', '')[:60]}")

    # 角色② 查询员:每个主题跑 agent loop
    print(f"\n[②查询员] 对 {len(topics)} 个主题跑 agent loop...")
    topics_data = []
    t2 = time.time()
    for i, t in enumerate(topics, 1):
        topic_name = t.get("topic", "")
        print(f"\n  --- 主题 {i}/{len(topics)}: {topic_name} ---")
        trace = research_topic_evolved(topic_name, lifeline_text)
        topic_entry = {
            "topic": topic_name,
            "data": trace.final_data,
            "loops": trace.loops,
            "tool_call_count": len(trace.tool_calls),
            "tool_calls": [
                {"step": tc.step, "thought": tc.thought, "tool": tc.tool, "args": tc.args, "result_preview": tc.result_preview}
                for tc in trace.tool_calls
            ],
            "error": trace.error,
        }
        topics_data.append(topic_entry)
        print(f"    {trace.loops}轮 {len(trace.tool_calls)}次调用 {'✅' if not trace.error else '⚠ ' + trace.error[:40]}")
        for tc in trace.tool_calls:
            dup = " [重复被拦]" if "[被拦" in tc.thought else ""
            print(f"    step{tc.step}: {tc.tool}({json.dumps(tc.args, ensure_ascii=False)}){dup}")
    result["research_sec"] = round(time.time() - t2, 1)
    result["topics_data"] = topics_data

    # 角色③ 分析员:结合演进脉络 + 数据,判断进展在演进中的意义
    print("\n[③分析员] 结合演进脉络+数据,判断进展在演进中的意义...")
    t3 = time.time()
    analysis = analyze_evolved_impact(lifeline_text, topics_data)
    result["analysis_sec"] = round(time.time() - t3, 1)
    result["analysis"] = analysis
    print(f"  耗时 {result['analysis_sec']}s")
    if "evolution_assessment" in analysis:
        print(f"\n  演进定位: {analysis['evolution_assessment'][:150]}")
    if "causal_chain" in analysis:
        print(f"  因果链: {analysis['causal_chain'][:150]}")
    if "overall" in analysis:
        print(f"  总结: {analysis['overall'][:150]}")
    for s in analysis.get("sectors", []):
        print(f"    - {s.get('sector','?')} [{s.get('judgment','?')}/{s.get('confidence','?')}]: {s.get('vs_history','')[:60]}")

    result["total_sec"] = round(result["etf_load_sec"] + result["interpret_sec"] + result["research_sec"] + result["analysis_sec"], 1)
    print(f"\n[完成] 总耗时 {result['total_sec']}s")
    return result


def write_report(results: list[dict]) -> None:
    lines = ["# 演进版 PoC 判定报告(消费持久话题演进脉络)", "",
             f"生成时间: {results[0].get('timestamp', '')}", f"模型: Qwen3.5-9B (llama.cpp @ localhost:8080)", "",
             "**核心区别**: 输入是持久话题的演进脉络(GetTopicLifeline 结构),不是孤立新闻。", "", "---", ""]
    for r in results:
        lines += [f"## {r['label']}", f"**话题**: {r['topic']}", "",
                  f"**总耗时**: {r.get('total_sec', '?')}s", "", "### ① 解读员(从演进脉络提炼需查主题)", ""]
        for t in r.get("interpret", {}).get("topics", []):
            lines.append(f"- **{t.get('topic', '?')}**: {t.get('reason', '')}")
        lines.append("")
        lines.append("### ② 查询员 agent loop")
        for td in r.get("topics_data", []):
            lines.append(f"#### {td['topic']} — {td['loops']}轮/{td['tool_call_count']}调用 {'✅' if not td['error'] else '⚠'}")
            for tc in td["tool_calls"]:
                lines.append(f"  step{tc['step']}: `{tc['tool']}({json.dumps(tc['args'], ensure_ascii=False)})`")
            lines.append(f"  数据: {td.get('data', '(空)')[:200]}")
            lines.append("")
        a = r.get("analysis", {})
        lines.append("### ③ 分析员(演进判断)")
        if a.get("evolution_assessment"):
            lines.append(f"\n**演进定位**: {a['evolution_assessment']}")
        if a.get("causal_chain"):
            lines.append(f"\n**因果链**: {a['causal_chain']}")
        for s in a.get("sectors", []):
            lines.append(f"- **{s.get('sector','?')}** [{s.get('judgment','?')}] 角色:{s.get('evolution_role','')} | 信号:{s.get('current_signal','')} | 演变:{s.get('vs_history','')}")
        if a.get("overall"):
            lines.append(f"\n> **总结**: {a['overall']}")
        lines += ["", "---", ""]
    with open("poc_evolved_report.md", "w", encoding="utf-8") as f:
        f.write("\n".join(lines))
    print("\n报告已写入 poc_evolved_report.md")


def main() -> None:
    results = [
        run_for_topic(MIDDLE_EAST_LIFELINE, label="中东地缘(4天演进)"),
        run_for_topic(LITHOGRAPHY_LIFELINE, label="国产光刻机(3天演进)"),
    ]
    write_report(results)


if __name__ == "__main__":
    main()
