"""模拟 GetTopicLifeline 的返回结构。

PoC 是 Python,不接 Go 的 DB。这里手造一个"中东地缘紧张"topic 积累 4 天的演进脉络,
结构严格对齐 Syntopica 的 GetTopicLifeline + DeriveSectionStatuses 输出:
  - topic 本体(label/description/计数器)
  - 按时间正序的 section nodes(每天一条,带 status: emerging/continuing/split/merge/ending)
  - relations(identity/similarity 边)

这个演进脉络是 agent 编排的真正输入。它体现的是"事件演进",不是单点新闻。
"""

from __future__ import annotations

# 模拟"中东地缘紧张"持久话题,积累 4 天的演进
MIDDLE_EAST_LIFELINE = {
    "topic": {
        "id": 1,
        "label": "中东地缘紧张与能源连锁反应",
        "description": "产油国设施遭袭 → 油价飙升 → 下游成本承压 + 避险情绪升温的连锁演进",
        "status": "active",
        "source": "auto",
        "first_seen_date": "2026-07-01",
        "last_seen_date": "2026-07-04",
        "hit_count": 4,
        "consecutive_hits": 4,
    },
    # 按时间正序的 section 节点 —— 每天一条,体现演进
    "sections": [
        {
            "section_id": 101,
            "period_date": "2026-07-01",
            "cluster_label": "产油国石油设施遭袭,原油产量短期下降",
            "status": "emerging",  # 首日涌现,无入边
            "topic_match_confidence": "auto_new",  # 新开 candidate
            "article_count": 8,
            "thread_count": 2,
            "thread_titles": [
                "某产油国主要石油设施遭袭,日产量下降15%",
                "布伦特原油单日涨幅超6%,市场恐慌情绪蔓延",
            ],
        },
        {
            "section_id": 102,
            "period_date": "2026-07-02",
            "cluster_label": "油价飙升传导:上游油气开采利好,下游化工航空成本承压",
            "status": "continuing",  # 稳接前日
            "topic_match_confidence": "anchor_hit",  # 双确认命中,演进连贯
            "article_count": 12,
            "thread_count": 3,
            "thread_titles": [
                "油气开采企业股价集体走高,市场预期业绩弹性",
                "化工行业面临原料成本上行压力,利润空间受挤压",
                "航空公司燃油成本占比攀升,多家下调盈利预期",
            ],
        },
        {
            "section_id": 103,
            "period_date": "2026-07-03",
            "cluster_label": "避险情绪升温推动贵金属走高 + 航运通道风险溢价",
            "status": "merge",  # 多个分支汇入 —— 演进在此合并了支线
            "topic_match_confidence": "anchor_hit",
            "article_count": 15,
            "thread_count": 3,
            "thread_titles": [
                "黄金价格突破阶段新高,避险资金涌入贵金属",
                "冲突区域靠近航运通道,多家航运公司加收风险附加费",
                "军工板块因地缘风险溢价上升走强",
            ],
        },
        {
            "section_id": 104,
            "period_date": "2026-07-04",
            "cluster_label": "冲突持续:光伏出海项目交付存疑,能源替代逻辑强化",
            "status": "continuing",  # 继续演进
            "topic_match_confidence": "anchor_hit",
            "article_count": 10,
            "thread_count": 2,
            "thread_titles": [
                "中东光伏项目进度受阻,出海企业订单交付存不确定性",
                "高油价强化能源自主替代逻辑,新能源板块获关注",
            ],
        },
    ],
    # 跨天关系边(模拟 RebuildBoardRelations 输出)
    "relations": [
        {"from": 101, "to": 102, "distance": 0.18, "relation_type": "identity"},  # 稳接
        {"from": 102, "to": 103, "distance": 0.24, "relation_type": "identity"},  # 稳接
        {"from": 103, "to": 104, "distance": 0.21, "relation_type": "identity"},  # 稳接
    ],
    # 当日(最新)的代表新闻片段 —— 给 agent 看"最新进展"的原文
    "latest_news_excerpt": (
        "新华社消息,近期中东地缘局势持续紧张。某产油国一处主要石油设施遭到袭击,"
        "导致原油日产量短期下降约15%。受此影响,国际油价大幅跳涨,布伦特原油单日涨幅超6%。"
        "市场担忧冲突区域靠近重要航运通道,多家航运公司调整航线。避险情绪推动黄金走高。"
        "分析人士提醒,若冲突升级,可能影响该地区光伏项目进度。"
    ),
}


# 对照:国产光刻机 topic,积累 3 天
LITHOGRAPHY_LIFELINE = {
    "topic": {
        "id": 2,
        "label": "国产光刻机自主化突破",
        "description": "国产芯片企业光刻机良率突破 → 半导体产业链自主化加速的演进",
        "status": "active",
        "source": "auto",
        "first_seen_date": "2026-07-02",
        "last_seen_date": "2026-07-04",
        "hit_count": 3,
        "consecutive_hits": 3,
    },
    "sections": [
        {
            "section_id": 201,
            "period_date": "2026-07-02",
            "cluster_label": "国产新一代光刻机研发取得突破,良率提升显著",
            "status": "emerging",
            "topic_match_confidence": "auto_new",
            "article_count": 6,
            "thread_count": 2,
            "thread_titles": [
                "某国产芯片企业宣布新一代光刻机良率突破",
                "已开始向多家国内晶圆厂小批量交付",
            ],
        },
        {
            "section_id": 202,
            "period_date": "2026-07-03",
            "cluster_label": "设备材料环节率先受益,国产替代预期升温",
            "status": "split",  # 演进分叉:一条主线,分出支线
            "topic_match_confidence": "anchor_hit",
            "article_count": 9,
            "thread_count": 2,
            "thread_titles": [
                "光刻机突破利好上游设备材料,订单预期增加",
                "半导体设备ETF资金净流入,市场博弈国产替代主线",
            ],
        },
        {
            "section_id": 203,
            "period_date": "2026-07-04",
            "cluster_label": "传统晶圆代工厂面临竞争压力,产业链格局重塑",
            "status": "continuing",
            "topic_match_confidence": "anchor_hit",
            "article_count": 7,
            "thread_count": 2,
            "thread_titles": [
                "依赖进口设备的传统代工厂面临竞争压力",
                "下游晶圆制造企业产能与良率有望提升",
            ],
        },
    ],
    "relations": [
        {"from": 201, "to": 202, "distance": 0.19, "relation_type": "identity"},
        {"from": 202, "to": 203, "distance": 0.23, "relation_type": "identity"},
    ],
    "latest_news_excerpt": (
        "某国产芯片企业宣布其新一代光刻机研发取得突破,良率提升显著,"
        "已开始向多家国内晶圆厂小批量交付。业内人士认为,这将加速国内半导体产业链自主化进程,"
        "利好上游设备材料和下游晶圆制造企业。同时,该进展可能对依赖进口设备的传统晶圆代工厂形成竞争压力。"
    ),
}


def render_lifeline_for_agent(lifeline: dict) -> str:
    """把 lifeline 渲染成 agent 能读的演进脉络文本。

    这是 agent 编排的核心输入 —— 不是单篇新闻,而是"这个话题怎么一步步演进到现在"。
    """
    t = lifeline["topic"]
    lines = [
        f"# 持久话题演进脉络",
        f"",
        f"## 话题本体",
        f"- 名称: {t['label']}",
        f"- 演进概述: {t['description']}",
        f"- 状态: {t['status']} | 首次出现: {t['first_seen_date']} | 最近: {t['last_seen_date']} | 累计命中: {t['hit_count']}天 | 连续: {t['consecutive_hits']}天",
        f"",
        f"## 逐日演进(按时间正序)",
    ]
    for s in lifeline["sections"]:
        conf_map = {"anchor_hit": "稳接", "auto_new": "新开", "unmatched": "断链", "manual": "人工接"}
        conf_cn = conf_map.get(s["topic_match_confidence"], s["topic_match_confidence"])
        status_cn = {"emerging": "涌现", "continuing": "延续", "split": "分叉", "merge": "合并", "ending": "结束"}.get(
            s["status"], s["status"]
        )
        lines.append(f"")
        lines.append(f"### {s['period_date']} [{status_cn}/{conf_cn}] {s['cluster_label']}")
        lines.append(f"  文章数: {s['article_count']} | 线索数: {s['thread_count']}")
        for title in s["thread_titles"]:
            lines.append(f"  - {title}")
    lines.append(f"")
    lines.append(f"## 最新进展原文(当日代表新闻)")
    lines.append(lifeline["latest_news_excerpt"])
    return "\n".join(lines)


if __name__ == "__main__":
    # 自测:看渲染出来的演进脉络长什么样
    print(render_lifeline_for_agent(MIDDLE_EAST_LIFELINE))
