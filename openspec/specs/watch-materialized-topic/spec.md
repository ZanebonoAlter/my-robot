## Purpose

关注标记（watch）从只读命中提示升级出两条"物化"轨：keyword_topic 把当天含关键字的文章聚合为固定名称话题 section，sentence_topic 用一句话向量检索当天相关辅助标签、聚合为可持续延续的持久话题 section。物化 section 与常规聚类 section 并排出现在日报中。

## Requirements

### Requirement: 关键字轨物化生成

系统 SHALL 在日报生成时，对 board 下每个 `status=active` 且 `type=keyword_topic` 的关注标记执行物化：扫描当天发布窗口内的全部未归档文章，按该关注的关键字表达式（复用既有 DNF 语义：`|` 分隔 OR 组、空白分隔 AND 词、大小写不敏感字面匹配）匹配，命中的全部文章聚合为一条固定名称 section（名称由关注的关键字表达式派生并可预期，如「关键字『harness』相关话题」），每篇命中文章对应一条 thread（标题 + 文本层摘要，机械组装，零 AI 调用）。

扫描的文本层 SHALL 为严格版：文章标题 + 摘要类字段择优（AI 内容摘要 > 正文抓取 > 原文 > 描述），SHALL NOT 匹配正文全文。

当天无命中文章时，该关注 SHALL NOT 产出 section（自然消失，非报错）。

#### Scenario: 含关键字文章聚合为固定话题

- **GIVEN** board #5 存在 active 的 keyword_topic 关注，表达式为 `harness`
- **WHEN** 当天有 3 篇未归档文章的标题或摘要含 "harness"（其中 1 篇无任何 event tag）
- **THEN** 当期日报 SHALL 产出一条固定名称 section，包含 3 条 thread（含无 tag 的那篇），thread 内容为机械组装，全程 SHALL NOT 发起 AI 调用

#### Scenario: 漏网文章可被捞回

- **GIVEN** 某篇文章未被任何 event tag 命中、也不属于任何聚类
- **WHEN** 其标题含关键字表达式全部词项
- **THEN** 该文章 SHALL 出现在关键字物化 section 中

#### Scenario: 无命中不产空 section

- **WHEN** 当天没有任何文章命中关键字表达式
- **THEN** 当期日报 SHALL NOT 出现该关注的物化 section

### Requirement: 一句话轨辅助标签检索

系统 SHALL 在关注创建时对 sentence_topic 的检索句生成一次 embedding 并缓存于关注记录；缓存缺失时（创建时生成失败、或检索句被更新后失效），下次日报生成 SHALL 惰性补算并回写。

日报生成时，系统 SHALL 用缓存的检索句向量在 board 绑定的辅助标签池（BoardComposition 关联的 SemanticLabel）内做余弦相似检索，取相似度不低于阈值（可配置）的 top-K 辅助标签为命中集；命中集经标签-tag 关联解析出 event tag，再限定为当天发布窗口内有文章的 tag，其文章并集构成物化文章集，聚合为一条 section（section 标题取关注的话题名）。

命中集为空、或解析后当天无文章时，该关注 SHALL NOT 产出 section。

#### Scenario: 检索命中并物化

- **GIVEN** board #5 存在 active 的 sentence_topic 关注（话题名「AI 编程工具进展」，检索句已缓存向量），辅助标签池中「AI 编程」标签向量与检索句余弦相似度超过阈值
- **WHEN** 「AI 编程」标签关联的 event tag 当天命中 4 篇文章
- **THEN** 当期日报 SHALL 产出标题为「AI 编程工具进展」的 section，包含这 4 篇文章

#### Scenario: 阈值过滤

- **WHEN** 辅助标签池中与检索句相似度最高的标签仍低于阈值
- **THEN** 该关注当期 SHALL NOT 产出 section

#### Scenario: 检索句更新后缓存失效

- **WHEN** 用户修改该关注的检索句
- **THEN** 系统 SHALL 使向量缓存失效；下次日报生成时用新检索句惰性补算并回写缓存

### Requirement: 一句话轨持久话题联动

sentence_topic 关注 SHALL 拥有一个专属持久话题（`source=manual`、`status=active`，首次物化时创建，label 取关注的话题名），其物化 section SHALL 归属该话题。该话题 SHALL 与普通手动话题一视同仁地参与后续日报的聚类锚定与自动归属，其生命周期（命中计数、连续命中、可见性）SHALL 遵循持久话题能力的既有规则推进，SHALL NOT 获得特殊阈值或豁免。

当期未产出物化 section 时，该话题 SHALL 按既有规则记为当日未命中（自然衰减，与普通话题一致）。

#### Scenario: 物化 section 推进话题延续

- **GIVEN** sentence_topic 关注的专属话题 T 已连续 2 天有物化 section
- **WHEN** 第 3 天物化 section 再次归属 T
- **THEN** T 的连续命中计数 SHALL 按持久话题既有规则 +1

#### Scenario: 物化话题作为聚类锚

- **GIVEN** sentence_topic 关注的专属话题 T 处于 active
- **WHEN** 后续日报聚类中某 event tag 的语义与 T 高度相近
- **THEN** 该 tag SHALL 按既有 lane 归属规则正常参与对 T 的归属判定，SHALL NOT 因 T 源自 watch 而被排除或放宽

#### Scenario: 无物化日自然衰减

- **WHEN** 某日检索句无命中、未产出物化 section
- **THEN** 专属话题 SHALL 按既有规则记当日未命中，SHALL NOT 保持虚假的连续命中

### Requirement: 物化 section 管线边界

物化 section SHALL 以 `lane_tier=watch_keyword`（关键字轨）或 `watch_sentence`（一句话轨）标记来源。物化 section SHALL NOT 参与同日 section 合并，SHALL NOT 参与 section 关系计算。section 自身的 article_count SHALL 如实反映其文章数；report 级聚合计数（article_count / event_tag_count / cluster_count）SHALL 保持常规聚类口径，SHALL NOT 因物化 section 重算。

#### Scenario: 不参与同日合并

- **GIVEN** 关键字物化 section 与某常规 section 语义高度相似
- **WHEN** 同日合并步骤执行
- **THEN** 物化 section SHALL NOT 被合并或改写

#### Scenario: 计数不重复

- **GIVEN** 某文章既在常规 section A 又在关键字物化 section W 中
- **WHEN** 日报保存
- **THEN** report 级 article_count SHALL 保持聚类口径不重复累计，section A 与 W 各自的 article_count SHALL 各自如实

### Requirement: 物化失败降级

任一物化轨在日报生成中失败（检索失败、数据解析失败等）SHALL 降级跳过该关注的当期物化并记录日志，SHALL NOT 阻断日报生成与保存，SHALL NOT 使日报状态置为失败。

#### Scenario: 单轨失败不阻断

- **WHEN** sentence_topic 检索调用失败
- **THEN** 该关注当期跳过，其余物化轨与日报流水线 SHALL 正常完成，日报 status SHALL 正常完成

### Requirement: 删除关注联动

删除 keyword_topic 关注 SHALL 仅停止后续物化，历史物化 section SHALL 保留原样。删除 sentence_topic 关注 SHALL 要求用户显式确认，确认后归档其专属持久话题（历史物化 section 保留，归属不变），SHALL NOT 静默归档。

#### Scenario: 删除关键字轨保留历史

- **WHEN** 用户删除 keyword_topic 关注
- **THEN** 后续日报不再产出该物化 section，已保存日报中的历史物化 section SHALL 保留

#### Scenario: 删除一句话轨确认归档

- **WHEN** 用户删除 sentence_topic 关注并确认
- **THEN** 其专属持久话题 SHALL 被归档（archived），历史物化 section 保留且归属不变

### Requirement: 物化轨不参与命中提示

命中提示判定 SHALL 跳过全部物化轨关注（keyword_topic / sentence_topic SHALL NOT 产生命中记录），物化 section SHALL NOT 被提示轨扫描命中。既有 label / keyword 提示轨行为 SHALL 保持不变。

#### Scenario: 物化轨无命中记录

- **WHEN** 日报生成完成，board 下存在 keyword_topic 关注
- **THEN** 该关注 SHALL NOT 产生任何命中提示记录
