"""三个角色 + agent loop。

角色①解读员:新闻原文 → 产业主题关键词清单
角色②查询员:主题 → 链式工具查询 → 数据(★核心验证对象,带 agent loop)
角色③分析员:新闻 + 数据 → 板块影响判断
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field

from llm_client import Message, chat, parse_json_response
from tools import TOOL_REGISTRY, execute_tool


# ===========================================================================
# 角色① 解读员
# ===========================================================================

def interpret_news(news: str) -> dict:
    """读新闻,提炼涉及的产业主题。

    返回:{"topics": [{"topic": "光刻机", "reason": "..."}, ...]}
    """
    prompt = f"""你是一位资深产业分析师。阅读下面这条新闻,提炼出它涉及的所有产业/行业主题。

要求:
- 每个主题用 2-6 字的简洁中文词表示(如:光刻机、半导体、光伏、锂电、军工、石油开采、航运)
- 给出判断该主题的依据
- 只提炼新闻明确涉及或强相关的产业,不要过度发散
- 输出 3-6 个主题为宜

输出严格 JSON,格式:
{{"topics": [{{"topic": "主题词", "reason": "新闻中提及...所以相关"}}]}}

新闻:
{news}"""
    resp = chat([Message("user", prompt)], temperature=0.2)
    parsed = parse_json_response(resp.content)
    if parsed and "topics" in parsed:
        return parsed
    return {"topics": [], "raw": resp.content, "error": "解析失败"}


# ===========================================================================
# 角色② 查询员(agent loop,核心验证对象)
# ===========================================================================

@dataclass
class ToolCall:
    """一次工具调用的完整记录,用于事后评判 agent 行为。"""
    step: int
    thought: str           # agent 决定调这个工具时的想法
    tool: str              # 工具名
    args: dict             # 参数
    result_preview: str    # 返回结果预览(截断)
    result_full: str       # 完整返回


@dataclass
class ResearcherTrace:
    """角色② 一次任务的完整 trace,判定报告的核心素材。"""
    topic: str
    tool_calls: list[ToolCall] = field(default_factory=list)
    final_data: str = ""          # 最终汇总给角色③的数据
    error: str = ""
    loops: int = 0


def _build_tools_desc() -> str:
    """把工具注册表渲染成 LLM 能看懂的描述。"""
    lines = []
    for name, spec in TOOL_REGISTRY.items():
        args_desc = json.dumps(spec["input_schema"], ensure_ascii=False)
        lines.append(f"- {name}: {spec['description']}\n  参数 schema: {args_desc}")
    return "\n".join(lines)


def research_topic(topic: str, max_loops: int = 6) -> ResearcherTrace:
    """角色②:针对单个产业主题,用 agent loop 查询相关数据。

    每一轮:
      1. 把(主题 + 历史工具调用 + 结果)喂给 LLM
      2. LLM 返回:要么调一个工具,要么宣布完成
      3. 执行工具,结果加进历史
      4. 循环,直到 LLM 宣布完成或达到 max_loops
    """
    trace = ResearcherTrace(topic=topic)
    tools_desc = _build_tools_desc()

    system = f"""你是一位 A 股数据查询员。你的任务:针对给定产业主题,使用工具查到相关的 ETF 实时行情数据。

可用工具:
{tools_desc}

工作流程(重要):
1. 先用 list_etf_by_keyword 用主题词查有没有对应 ETF
2. 如果命中很少(0-1 个),换更宽泛或相关的产业词重查(例如"光刻机"→"半导体"/"芯片")。最多换 2-3 个词,换不出来就基于已有的查
3. 拿到 ETF 代码后,用 get_etf_quote 查实时行情
4. 拿到行情数据后,立即宣布完成

关键纪律(违反会导致死循环):
- 工具返回的数据是完整的。JSON 里 total_count 就是真实命中数,返回的 etfs 列表就是全部。如果看起来"显示不全",那是显示问题不是数据缺失,基于已有代码继续下一步,绝对不要重查同一个关键词。
- 查行情时取 3-5 只代表性 ETF 即可,不需要获取全部代码。一个主题拿到几个有代表性的代码+行情就够了。
- 绝对不要用相同参数重复调用同一个工具。已经查过的关键词不要再查。

每一轮你必须输出严格 JSON,二选一:
- 继续调工具:{{"action": "call_tool", "thought": "我接下来要...因为...", "tool": "工具名", "args": {{...}}}}
- 宣布完成:{{"action": "finish", "thought": "我已经查到了...", "summary": "给分析师的简明数据汇总,包含查到的 ETF 及涨跌情况"}}

不要输出 JSON 以外的任何内容。"""

    history_lines: list[str] = []  # 累积的工具调用历史,作为多轮上下文

    for step in range(1, max_loops + 1):
        trace.loops = step
        # 构造上下文:系统提示 + 主题 + 历史调用 + "下一步"
        history_block = "\n".join(history_lines) if history_lines else "(尚无工具调用)"
        user_msg = f"""/no_think
当前要研究的产业主题: {topic}

已有的工具调用历史:
{history_block}

请决定下一步(调工具或宣布完成),输出 JSON。"""
        resp = chat(
            [Message("system", system), Message("user", user_msg)],
            temperature=0.2,
        )
        decision = parse_json_response(resp.content)
        if not decision:
            trace.error = f"第{step}轮 LLM 输出无法解析为 JSON: {resp.content[:300]}"
            break

        action = decision.get("action")
        thought = decision.get("thought", "")

        if action == "finish":
            trace.final_data = decision.get("summary", "")
            history_lines.append(f"第{step}步: 宣布完成 — {thought}")
            break

        if action == "call_tool":
            tool_name = decision.get("tool", "")
            args = decision.get("args", {}) or {}
            # 去重防御:相同工具+参数直接拦截,返回提示,避免死循环
            call_key = (tool_name, json.dumps(args, sort_keys=True, ensure_ascii=False))
            seen_calls = {(tc.tool, json.dumps(tc.args, sort_keys=True, ensure_ascii=False)) for tc in trace.tool_calls}
            if call_key in seen_calls:
                result = json.dumps(
                    {"error": f"你已经用相同参数调用过 {tool_name},结果见历史。不要重复调用,基于已有数据继续下一步或宣布完成。"},
                    ensure_ascii=False,
                )
                trace.tool_calls.append(ToolCall(
                    step=step, thought=thought + " [被拦截:重复调用]", tool=tool_name, args=args,
                    result_preview=result, result_full=result,
                ))
                history_lines.append(f"第{step}步: 调用 {tool_name}({json.dumps(args, ensure_ascii=False)}) — 结果: {result}")
                continue
            result = execute_tool(tool_name, args)
            tc = ToolCall(
                step=step,
                thought=thought,
                tool=tool_name,
                args=args,
                result_preview=result[:300],
                result_full=result,
            )
            trace.tool_calls.append(tc)
            # 关键:给完整结果,不要截断,否则 agent 误以为"没拿全"而死循环重查
            history_lines.append(
                f"第{step}步: 调用 {tool_name}({json.dumps(args, ensure_ascii=False)}) — 想法: {thought} — 结果: {result}"
            )
            continue

        trace.error = f"第{step}轮 action 字段不合法: {action}"
        break
    else:
        trace.error = f"达到最大循环数 {max_loops} 未完成"

    return trace


# ===========================================================================
# 角色③ 分析员
# ===========================================================================

def analyze_impact(news: str, topics_data: list[dict]) -> dict:
    """结合新闻 + 各主题的查询数据,判断板块影响。

    topics_data: [{"topic": "...", "data": "...(查询员 summary)", "trace_loops": n, "tool_calls": m}]
    返回:{"affected_sectors": [...], "analysis": "..."}
    """
    topics_block = "\n\n".join(
        f"【主题: {t['topic']}】\n查询到的数据:\n{t.get('data', '(无数据)')}"
        for t in topics_data
    )
    prompt = f"""你是一位资深 A 股策略分析师。基于以下新闻和补充查到的 ETF 行情数据,分析哪些板块会受影响、影响方向如何。

新闻:
{news}

各产业主题的实时数据:
{topics_block}

请输出严格 JSON:
{{"affected_sectors": [
  {{"sector": "板块/主题名", "direction": "利好/利空/中性", "confidence": "高/中/低", "reasoning": "结合新闻和数据,为什么判断这个板块受这个方向的影响", "evidence_etf": "支撑判断的关键 ETF 及涨跌数据"}}
],
"overall": "一句话总结整体判断"}}"""
    resp = chat([Message("user", prompt)], temperature=0.3)
    parsed = parse_json_response(resp.content)
    if parsed and "affected_sectors" in parsed:
        return parsed
    return {"affected_sectors": [], "raw": resp.content, "error": "解析失败"}
