## ADDED Requirements

### Requirement: 实体+数值提取器

系统 SHALL 提供 `ExtractEntities(label string) *LabelEntities` 函数，从标签文本中提取结构化信息。

#### 返回结构
```go
type LabelEntities struct {
    Numbers   []string  // 提取的数值（归一化后），如 ["1.5万亿", "750亿", "48.2%"]
    Keywords  []string  // 关键词（去停用词后的切分结果），如 ["沪深", "成交额", "突破", "万亿"]
    Acronyms  []string  // 大写英文缩写/产品名，如 ["SpaceX", "S-1", "AI", "Grok"]
}
```

#### Scenario: 提取数值
- **WHEN** label = `"沪深两市成交额突破1.5万亿元"`
- **THEN** `Numbers` = `["1.5万亿"]`

#### Scenario: 提取百分比
- **WHEN** label = `"问界汽车5月全系车型交付量环比增长48.2%"`
- **THEN** `Numbers` = `["48.2%"]`

#### Scenario: 提取英文缩写
- **WHEN** label = `"SpaceX 公开 S-1 招股书计划募资 750 亿美元"`
- **THEN** `Acronyms` = `["SpaceX", "S-1"]`
- **AND** `Numbers` = `["750亿"]`

#### Scenario: 提取关键词
- **WHEN** label = `"山西沁源煤矿瓦斯爆炸事故"`
- **THEN** `Keywords` 包含 `["山西", "沁源", "煤矿", "瓦斯", "爆炸"]`（子集）

#### Scenario: 纯中文短标签
- **WHEN** label = `"华虹半导体"`
- **THEN** `Numbers` = `[]`
- **AND** `Acronyms` = `[]`
- **AND** `Keywords` 包含 `["华虹", "半导体"]`

### Requirement: 候选对过滤函数

系统 SHALL 提供 `ShouldConsiderMerge(labelA, labelB string) (bool, string)` 函数，判断两个标签是否值得作为合并候选。

#### 过滤规则
1. 提取 `entA = ExtractEntities(labelA)`, `entB = ExtractEntities(labelB)`
2. 如果 `entA.Numbers` 和 `entB.Numbers` 都非空且**不相等**（集合比较）→ **REJECT**，原因 `"numeric_mismatch"`
3. 如果 `entA.Acronyms ∪ entA.Keywords` 与 `entB.Acronyms ∪ entB.Keywords` 的交集为空 → **REJECT**，原因 `"no_common_entities"`
4. 其他情况 → **PASS**

#### Scenario: 数值不同 → REJECT
- **WHEN** labelA = `"沪深两市成交额突破1万亿元"`, labelB = `"沪深两市成交额突破1.5万亿元"`
- **THEN** 返回 `(false, "numeric_mismatch")`

#### Scenario: 数值相同 → PASS
- **WHEN** labelA = `"小鹏集团5月交付新车32158辆"`, labelB = `"小鹏集团2026年5月交付新车32,158辆"`
- **THEN** 返回 `(true, "")`
- **AND** `Numbers` 都提取为 `["32158"]`（逗号去除后归一化）

#### Scenario: 无共同实体 → REJECT
- **WHEN** labelA = `"米拉诺维奇"`, labelB = `"涅边贾"`
- **THEN** 返回 `(false, "no_common_entities")`

#### Scenario: 有共同实体 → PASS
- **WHEN** labelA = `"SpaceX 公开 S-1 招股书"`, labelB = `"SpaceX首次公开募股"`
- **THEN** 返回 `(true, "")`（共同实体: "SpaceX"）

#### Scenario: 双方无数值无交集 → PASS（保守策略）
- **WHEN** labelA = `"AI 短剧行业爆发与内卷并存"`, labelB = `"AI 短剧出海遇冷"`
- **THEN** 返回 `(true, "")`（有共同关键词 "AI", "短剧"）

#### Scenario: 单方有数值 → PASS
- **WHEN** labelA = `"戴尔大涨近33%创最大单日涨幅"`, labelB = `"戴尔科技美股盘前大涨"`
- **THEN** 返回 `(true, "")`（labelB 无数值，不触发 numeric_mismatch）
