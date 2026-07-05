"""数据源工具层。

每个工具就是 agent 可调用的一个查询能力。带 name/description/input_schema 三件套,
这是让 LLM 理解"我能调什么"的标准格式。

数据源:akshare(东方财富)+ 新浪财经,全部免费 HTTP。
"""

from __future__ import annotations

import json
from typing import Any

import akshare as ak
import requests

# ETF 全量行情的全局缓存:一次日报只拉一遍全市场,避免反复翻 15 页
_ETF_SPOT_CACHE: list[dict[str, Any]] | None = None


def _load_etf_spot() -> list[dict[str, Any]]:
    """加载全市场 ETF 实时行情,缓存复用。返回扁平 dict 列表。"""
    global _ETF_SPOT_CACHE
    if _ETF_SPOT_CACHE is not None:
        return _ETF_SPOT_CACHE
    df = ak.fund_etf_spot_em()
    # 只保留 agent 需要看到的字段,降低 token 消耗
    cols = ["代码", "名称", "最新价", "涨跌幅", "成交额"]
    _ETF_SPOT_CACHE = df[cols].fillna("").to_dict(orient="records")
    return _ETF_SPOT_CACHE


# ---------------------------------------------------------------------------
# 工具实现
# ---------------------------------------------------------------------------


def list_etf_by_keyword(keyword: str) -> str:
    """按名称关键词筛选全市场 ETF,返回命中清单。

    作为"目录查询"工具:agent 拿到一个产业主题(如"光刻机"),
    先用它找到有哪些相关 ETF 及其代码。
    """
    spot = _load_etf_spot()
    keyword = keyword.strip()
    hits = [r for r in spot if keyword in str(r.get("名称", ""))]
    if not hits:
        return json.dumps(
            {"hit_count": 0, "hint": f"没有名称含'{keyword}'的 ETF,建议换更宽泛的产业关键词重试"},
            ensure_ascii=False,
        )
    # 返回全部命中,但只保留必要字段控制 token。
    # 关键:不截断数量,只精简字段,避免 agent 误以为"没拿全"而反复重查。
    slim = [{"代码": r["代码"], "名称": r["名称"], "涨跌幅": r.get("涨跌幅", "")} for r in hits]
    return json.dumps(
        {"total_count": len(slim), "etfs": slim, "note": "已返回全部命中,无需重查"},
        ensure_ascii=False,
    )


def get_etf_quote(codes: list[str]) -> str:
    """获取指定 ETF 代码的实时行情(新浪接口,~200ms)。

    作为"详情查询"工具:agent 通过 list_etf_by_keyword 拿到代码后,
    用它取实时涨跌数据。
    """
    if not codes:
        return json.dumps({"error": "codes 为空"}, ensure_ascii=False)
    # 新浪支持批量查询,代码用逗号拼接,sh/sz 前缀按首数字判断
    prefixed = []
    for c in codes:
        c = c.strip()
        if not c:
            continue
        prefix = "sh" if c.startswith(("5", "6", "9")) else "sz"
        prefixed.append(f"{prefix}{c}")
    if not prefixed:
        return json.dumps({"error": "无有效代码"}, ensure_ascii=False)
    url = f"http://hq.sinajs.cn/list={','.join(prefixed)}"
    headers = {"Referer": "https://finance.sina.com.cn"}
    try:
        resp = requests.get(url, headers=headers, timeout=5)
        resp.encoding = "gbk"
    except Exception as e:
        return json.dumps({"error": f"请求失败: {e}"}, ensure_ascii=False)
    results = []
    for line in resp.text.strip().split("\n"):
        if "=" not in line:
            continue
        code_part = line.split("=")[0].split("_")[-1].split(".")[-1]
        content = line.split('"')[1] if '"' in line else ""
        fields = content.split(",")
        if len(fields) < 4:
            continue
        name = fields[0]
        price = fields[1]
        prev_close = fields[2]
        try:
            chg_pct = round((float(price) - float(prev_close)) / float(prev_close) * 100, 2) if float(prev_close) else 0.0
        except (ValueError, ZeroDivisionError):
            chg_pct = 0.0
        results.append({"code": code_part, "name": name, "price": price, "chg_pct": chg_pct})
    return json.dumps({"quotes": results}, ensure_ascii=False)


def list_sectors() -> str:
    """列出东方财富行业板块清单。帮 agent 建立"有哪些板块"的意识。"""
    try:
        df = ak.stock_board_industry_name_em()
    except Exception as e:
        return json.dumps({"error": f"获取板块失败: {e}"}, ensure_ascii=False)
    cols = ["板块名称", "最新价", "涨跌幅"]
    records = df[cols].fillna("").to_dict(orient="records")
    return json.dumps({"sector_count": len(records), "sectors": records[:30]}, ensure_ascii=False)


# ---------------------------------------------------------------------------
# 工具注册表:agent 看到的接口
# ---------------------------------------------------------------------------

TOOL_REGISTRY = {
    "list_etf_by_keyword": {
        "description": "按名称关键词筛选全市场 ETF,返回命中的 ETF 代码和名称清单。用于从产业主题找到对应的 ETF 标的。当一个关键词命中很少时,应该换更宽泛或相关的产业词重试。",
        "input_schema": {"type": "object", "properties": {"keyword": {"type": "string", "description": "ETF 名称关键词,如 半导体、芯片、光伏、军工、白酒、医药、新能源"}},
                         "required": ["keyword"]},
        "fn": list_etf_by_keyword,
    },
    "get_etf_quote": {
        "description": "获取指定 ETF 代码列表的实时行情(最新价、涨跌幅)。需要先通过 list_etf_by_keyword 拿到代码。",
        "input_schema": {"type": "object", "properties": {"codes": {"type": "array", "items": {"type": "string"}, "description": "ETF 代码列表,如 ['512480', '588000']"}},
                         "required": ["codes"]},
        "fn": get_etf_quote,
    },
    "list_sectors": {
        "description": "列出 A 股全部行业板块及其当日涨跌幅。当不确定某产业对应哪些板块时使用。",
        "input_schema": {"type": "object", "properties": {}, "required": []},
        "fn": list_sectors,
    },
}


def execute_tool(name: str, args: dict[str, Any]) -> str:
    """执行工具调用。未知工具或参数错误返回 JSON error。"""
    tool = TOOL_REGISTRY.get(name)
    if tool is None:
        return json.dumps({"error": f"未知工具: {name}, 可用: {list(TOOL_REGISTRY)}"}, ensure_ascii=False)
    try:
        return tool["fn"](**args)
    except TypeError as e:
        return json.dumps({"error": f"参数错误: {e}"}, ensure_ascii=False)
    except Exception as e:
        return json.dumps({"error": f"执行失败: {e}"}, ensure_ascii=False)
