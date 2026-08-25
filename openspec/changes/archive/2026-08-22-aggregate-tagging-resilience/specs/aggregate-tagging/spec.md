# aggregate-tagging Delta Spec

## MODIFIED Requirements

### Requirement: 逐片融合提取标签

聚合路径 SHALL 对每个栏目片发起恰好 1 次 LLM 调用，使用 event/person/keyword 三分类融合的单一 prompt（每片产出上限 4 个标签候选），SHALL NOT 对单篇文章发起超过 8 次提取调用。

单片调用失败时 SHALL 记录 warning 并跳过该片继续处理其余片，SHALL NOT 因单片失败丢弃整篇或整篇重试。

单个标签的 auxiliary_labels 校验失败或 keyword description 缺失 SHALL 降级处理（丢弃校验不过的部分、保留其余标签），SHALL NOT 因单标签问题让整片候选报废；每次降级 SHALL 记录 warning（含标签名与原因）。JSON 整体解析失败 SHALL 维持原有重试语义（重试 3 次后跳过该片）。

#### Scenario: 每片一次调用产出候选

- **WHEN** 切片器产出 5 个栏目片
- **THEN** 发起 5 次融合 prompt 提取调用，每次调用的输入仅含对应片文本

#### Scenario: 单片失败不影响其余片

- **WHEN** 5 片中第 3 片的 LLM 调用重试耗尽后仍失败
- **THEN** 第 1、2、4、5 片的标签候选正常进入 reduce 阶段，失败信息记录在日志

#### Scenario: 单标签 aux 校验失败降级保留

- **WHEN** 某片返回 3 个标签，其中 1 个 event 标签的 auxiliary_labels 数量为 2（不满足 3-5 条）
- **THEN** 该 event 标签仍入库（无 aux 锚点），同片其余 2 个标签不受影响，降级记录在 warning 日志

#### Scenario: keyword 缺 description 单标签跳过

- **WHEN** 某片返回的 keyword 标签 description 为空
- **THEN** 该 keyword 标签被跳过并记录 warning，同片其余标签正常进入 reduce 阶段

### Requirement: 跨片去重与文章级上限

聚合路径 SHALL 以纯代码方式跨片去重（按 `Slugify(label)`，重复时保留首栏目出现者）并将文章级标签上限设为 15。语义级撞车（不同措辞指同一话题）SHALL 交由既有标签合并建议机制处理，SHALL NOT 在本次提取链路中新增 LLM 仲裁调用。

聚合路径全部片处理完成后标签数为 0（全片失败或全部空产出）时 SHALL 回落 mono 提取路径（双分支 LLM 提取，含 heuristic 兜底），SHALL NOT 让聚合型文章以 0 标签结束打标。

#### Scenario: 同名标签跨片去重

- **WHEN** "工具推荐"片与"正文整理"片产出了同名标签 "Kubernetes"
- **THEN** 该标签只保留一份，归属首栏目出现的片（score 更高）

#### Scenario: 文章级上限截断

- **WHEN** reduce 后候选标签共 17 个
- **THEN** 按片顺序与片内优先级保留前 15 个入库

#### Scenario: 全片失败回落 mono 路径

- **WHEN** 某聚合文章的所有栏目片提取均失败（或全部返回空候选）
- **THEN** 该文章走 mono 双分支提取路径打标，回落原因记录在日志，最终标签数不为 0（除非 mono 路径同样无产出）

## ADDED Requirements

### Requirement: 融合提取 JSON 尾逗号容错

融合与 mono 提取共用的 JSON 标签解析入口（`parseRawTagObjects`）SHALL 在反序列化前对输入做无损修复：剥除对象与数组末元素后的多余逗号；修复 SHALL NOT 影响合法 JSON 的解析结果，SHALL NOT 修改字符串字面量内部内容。

#### Scenario: 尾逗号 JSON 被修复解析

- **WHEN** 模型返回的 tags JSON 含 `"label": "X",}` 形式的尾逗号
- **THEN** 修复后解析成功，标签 "X" 正常产出，不触发重试

#### Scenario: 合法 JSON 不受影响

- **WHEN** 模型返回标准合法 JSON
- **THEN** 解析行为与修复前完全一致

#### Scenario: 字符串内逗号结构不误伤

- **WHEN** 标签 description 字符串字面量内容含 `,}` 字符序列
- **THEN** 该字符串内容保持原样，不被修复函数改动
