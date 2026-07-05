"""演进版三角色 —— 消费持久话题的演进脉络(而非孤立新闻)。

核心区别 vs 第一版:
  旧版: run_analysis(news) → 单点分析,看不见演进
  新版: run_analysis_for_topic(lifeline) → 带着演进脉络查数据,判断进展在演进中的意义

三角色职责调整:
  ①解读员: 从演进脉络提炼"需要补哪些实时数据来佐证/丰富演进判断"
  ②查询员: agent loop 查数据(不变,仍是核心验证对象)
  ③分析员: 结合演进脉络 + 数据,判断"最新进展在这个话题演进里意味着什么"
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field

from llm_client import Message, chat, parse_json_response
from lifeline_mock import render_lifeline_for_agent
from roles import ToolCall, _build_tools_desc  # 复用第一版的工具调用骨架
from tools import TOOL_REGISTRY, execute_tool


# ===========================================================================
# 角色① 解读员(演进版):读演进脉络,提炼需要补数据的产业主题
# ===========================================================================

def interpret_lifeline(lifeline_text: str) -> dict:
    """读演进脉络,提炼"需要查哪些产业板块的实时数据"来佐证演进判断。

    关键:不是泛泛提炼主题,而是带着"这个话题在演进、最新进展是什么"的视角,
    判断"哪些板块的数据能帮我们看清这次进展在演进中的位置"。
    """
    prompt = f"""你是一位资深产业分析师。下面是一个持久话题的演进脉络(跨多天的事件演进)。

你的任务:基于这个演进脉络,提炼出**需要查询哪些产业/板块的实时行情数据**,以便佐证或丰富对"这次最新进展在整个演进里意味着什么"的判断。

要求:
- 主题必须是 A 股有对应 ETF 的产业方向(如:石油/能源、黄金/贵金属、军工、航空、航运/物流、光伏/新能源、化工、半导体等)
- 每个主题给出"为什么要查它"的理由(关联到演进脉络的哪一天/哪个环节)
- 提炼 3-5 个主题,聚焦最能佐证演进判断的方向

输出严格 JSON:
{{"topics": [{{"topic": "产业主题词", "reason": "关联演进:...所以需要查它的实时表现"}}]}}

---
{lifeline_text}"""
    resp = chat([Message("user", prompt)], temperature=0.2)
    parsed = parse_json_response(resp.content)
    if parsed and "topics" in parsed:
        return parsed
    return {"topics": [], "raw": resp.content, "error": "解析失败"}


# ===========================================================================
# 角色② 查询员(演进版):复用第一版 agent loop,签名不变
# ===========================================================================

def research_topic_evolved(topic: str, lifeline_text: str, max_loops: int = 6):
    """演进版查询员。比第一版多一个输入:演进脉络(作为查询的背景上下文)。

    这样 agent 查数据时带着"这个话题在演进"的意识,而不是孤立地查。
    """
    from roles import ResearcherTrace

    trace = ResearcherTrace(topic=topic)
    tools_desc = _build_tools_desc()

    system = f"""你是一位 A 股数据查询员。背景:有一个持久话题正在演进(见下方脉络),你需要针对给定的产业主题,查到相关的 ETF 实时行情数据,帮助分析"最新进展在这个演进里的意义"。

可用工具:
{tools_desc}

工作流程(重要):
1. 先用 list_etf_by_keyword 用主题词查有没有对应 ETF
2. 如果命中很少(0-1 个),换更宽泛或相关的产业词重查(例如"光刻机"→"半导体"/"芯片")。最多换 2-3 个词
3. 拿到 ETF 代码后,用 get_etf_quote 查实时行情,取 3-5 只代表性 ETF 即可
4. 拿到行情数据后,立即宣布完成

关键纪律(违反会导致死循环):
- 工具返回的数据是完整的。total_count 就是真实命中数,不要因为"看起来不全"重查同一个关键词
- 查行情取 3-5 只代表性 ETF 即可,不需要全部代码
- 绝对不要用相同参数重复调用同一个工具

每一轮输出严格 JSON,二选一:
- 继续调工具:{{"action": "call_tool", "thought": "...", "tool": "工具名", "args": {{...}}}}
- 宣布完成:{{"action": "finish", "thought": "...", "summary": "给分析师的简明数据汇总"}}

不要输出 JSON 以外的任何内容。

话题演进脉络(背景):
{lifeline_text}"""

    history_lines: list[str] = []

    for step in range(1, max_loops + 1):
        trace.loops = step
        history_block = "\n".join(history_lines) if history_lines else "(尚无工具调用)"
        user_msg = f"""/no_think
当前要查询的产业主题: {topic}

已有的工具调用历史:
{history_block}

请决定下一步(调工具或宣布完成),输出 JSON。"""
        resp = chat([Message("system", system), Message("user", user_msg)], temperature=0.2)
        decision = parse_json_response(resp.content)
        if not decision:
            trace.error = f"第{step}轮 LLM 输出无法解析: {resp.content[:200]}"
            break

        action = decision.get("action")
        thought = decision.get("thought", "")

        if action == "finish":
            trace.final_data = decision.get("summary", "")
            break

        if action == "call_tool":
            tool_name = decision.get("tool", "")
            args = decision.get("args", {}) or {}
            call_key = (tool_name, json.dumps(args, sort_keys=True, ensure_ascii=False))
            seen_calls = {(tc.tool, json.dumps(tc.args, sort_keys=True, ensure_ascii=False)) for tc in trace.tool_calls}
            if call_key in seen_calls:
                result = json.dumps({"error": f"已用相同参数调用过 {tool_name},不要重复,基于已有数据继续。"}, ensure_ascii=False)
                trace.tool_calls.append(ToolCall(step=step, thought=thought + " [被拦:重复]", tool=tool_name, args=args, result_preview=result, result_full=result))
                history_lines.append(f"第{step}步: 调用 {tool_name}({json.dumps(args, ensure_ascii=False)}) — 结果: {result}")
                continue
            result = execute_tool(tool_name, args)
            tc = ToolCall(step=step, thought=thought, tool=tool_name, args=args, result_preview=result[:300], result_full=result)
            trace.tool_calls.append(tc)
            history_lines.append(f"第{step}步: 调用 {tool_name}({json.dumps(args, ensure_ascii=False)}) — 想法: {thought} — 结果: {result}")
            continue

        trace.error = f"第{step}轮 action 不合法: {action}"
        break
    else:
        trace.error = f"达到最大循环数 {max_loops} 未完成"

    return trace


# ===========================================================================
# 角色③ 分析员(演进版):结合演进脉络 + 数据,判断进展在演进中的意义
# ===========================================================================

def analyze_evolved_impact(lifeline_text: str, topics_data: list[dict]) -> dict:
    """结合演进脉络 + 实时数据,判断"最新进展在这个话题演进里意味着什么"。

    关键区别 vs 第一版:不是孤立判断"哪些板块受影响",
    而是判断"这次进展在 4 天的演进里处于什么位置、强化还是转折了既有趋势"。
    """
    topics_block = "\n\n".join(
        f"【{t['topic']}】\n查询数据:\n{t.get('data', '(无数据)')}" for t in topics_data
    )
    prompt = f"""你是一位资深 A 股策略分析师。下面是一个持久话题的完整演进脉络,以及补充查到的 ETF 实时行情数据。

你的任务:**结合演进脉络和实时数据,判断最新进展在这个话题演进里意味着什么**。

分析要求:
- 不要孤立地判断"利好/利空",而要回答:这次进展是**强化了既有趋势**,还是**出现了转折/扩散**?
- 引用演进脉络里具体哪一天的线索作为对比基准(比如"相比7-02的化工承压,7-04的数据显示...")
- 用查到的实时行情数据佐证你的判断(具体到 ETF 涨跌)
- 识别演进中的**因果链**(油价飙升 → 哪些板块连锁反应)
- 如有数据与演进叙事矛盾的,明确指出

输出严格 JSON:
{{"evolution_assessment": "最新进展在演进中的定位(强化/转折/扩散 + 理由)",
 "sectors": [
   {{"sector": "...", "evolution_role": "在因果链中的位置(源头/传导/末端)", "current_signal": "实时数据给出的信号", "vs_history": "相比前几日的演变", "judgment": "利好/利空/中性", "confidence": "高/中/低"}}
 ],
 "causal_chain": "演进中的因果链描述",
 "overall": "一句话总结这次进展在整个演进里的意义"}}"""

    prompt += f"\n\n---\n话题演进脉络:\n{lifeline_text}\n\n各主题实时数据:\n{topics_block}"
    resp = chat([Message("user", prompt)], temperature=0.3)
    parsed = parse_json_response(resp.content)
    if parsed and ("sectors" in parsed or "evolution_assessment" in parsed):
        return parsed
    return {"raw": resp.content, "error": "解析失败"}
