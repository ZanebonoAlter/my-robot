"""编排器:把三个角色串成一条完整管线。

管线:
  新闻 → ①解读员(提炼主题) → ②查询员×N(每个主题跑 agent loop) → ③分析员(出板块影响判断)
"""

from __future__ import annotations

import json
import time
from datetime import datetime

from roles import analyze_impact, interpret_news, research_topic
from tools import _load_etf_spot  # 预热缓存


def run_analysis(news: str, label: str = "") -> dict:
    """跑一条新闻的完整分析管线,返回完整 trace。"""
    print(f"\n{'='*70}")
    print(f"开始分析 [{label}]")
    print(f"{'='*70}")

    result = {"label": label, "news": news, "timestamp": datetime.now().isoformat()}

    # --- 预热 ETF 缓存(一次日报只拉一遍全市场) ---
    t0 = time.time()
    print("\n[预热] 加载全市场 ETF 行情...")
    _load_etf_spot()
    result["etf_load_sec"] = round(time.time() - t0, 1)
    print(f"  完成,耗时 {result['etf_load_sec']}s")

    # --- 角色① 解读员 ---
    print("\n[①解读员] 提炼产业主题...")
    t1 = time.time()
    interp = interpret_news(news)
    result["interpret_sec"] = round(time.time() - t1, 1)
    topics = interp.get("topics", [])
    result["interpret"] = interp
    print(f"  提炼出 {len(topics)} 个主题,耗时 {result['interpret_sec']}s:")
    for t in topics:
        print(f"    - {t.get('topic', '?')}: {t.get('reason', '')[:50]}")

    # --- 角色② 查询员(每个主题跑 agent loop) ---
    print(f"\n[②查询员] 对 {len(topics)} 个主题跑 agent loop...")
    topics_data = []
    t2 = time.time()
    for i, t in enumerate(topics, 1):
        topic_name = t.get("topic", "")
        print(f"\n  --- 主题 {i}/{len(topics)}: {topic_name} ---")
        trace = research_topic(topic_name)
        topic_entry = {
            "topic": topic_name,
            "data": trace.final_data,
            "loops": trace.loops,
            "tool_call_count": len(trace.tool_calls),
            "tool_calls": [
                {
                    "step": tc.step,
                    "thought": tc.thought,
                    "tool": tc.tool,
                    "args": tc.args,
                    "result_preview": tc.result_preview,
                }
                for tc in trace.tool_calls
            ],
            "error": trace.error,
        }
        topics_data.append(topic_entry)
        print(f"    循环 {trace.loops} 次,工具调用 {len(trace.tool_calls)} 次")
        if trace.error:
            print(f"    ⚠ 错误: {trace.error}")
        for tc in trace.tool_calls:
            print(f"    step{tc.step}: {tc.tool}({json.dumps(tc.args, ensure_ascii=False)})")
    result["research_sec"] = round(time.time() - t2, 1)
    result["topics_data"] = topics_data
    print(f"\n  查询员总耗时 {result['research_sec']}s")

    # --- 角色③ 分析员 ---
    print("\n[③分析员] 综合判断板块影响...")
    t3 = time.time()
    analysis = analyze_impact(news, topics_data)
    result["analysis_sec"] = round(time.time() - t3, 1)
    result["analysis"] = analysis
    sectors = analysis.get("affected_sectors", [])
    print(f"  判断 {len(sectors)} 个受影响板块,耗时 {result['analysis_sec']}s:")
    for s in sectors:
        print(f"    - {s.get('sector', '?')} [{s.get('direction', '?')}/{s.get('confidence', '?')}]: {s.get('reasoning', '')[:60]}")
    print(f"  总体: {analysis.get('overall', '')}")

    result["total_sec"] = round(result["etf_load_sec"] + result["interpret_sec"] + result["research_sec"] + result["analysis_sec"], 1)
    print(f"\n[完成] 总耗时 {result['total_sec']}s")
    return result
