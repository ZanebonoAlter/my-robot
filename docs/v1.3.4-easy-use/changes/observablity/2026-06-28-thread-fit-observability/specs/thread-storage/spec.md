## ADDED Requirements

### Requirement: Thread 贴合度信号存储

`daily_report_threads` 表 SHALL 新增两列存储 thread↔section 标题贴合度信号：`embedding`（thread 标题向量，`vector(2560)`，与 section 标题 embedding 同维度同 provider）与 `fit_distance`（该 thread 标题 embedding 与其所属 section 标题 embedding 的余弦距离，`numeric`，越小越贴合）。两列均可空（历史 thread 与 embedding 失败的 thread 留 NULL）。

`DailyReportThread` GORM 模型 SHALL 新增 `Embedding string`（`gorm:"type:vector" json:"-"`，绝不外泄到 API）与 `FitDistance float64`（`json:"fit_distance,omitempty"`）字段。通过 detail/timeline/lifeline/topic-lifeline 接口的 section.Threads GORM Preload，`fit_distance` SHALL 自动随 thread 返回前端；`embedding` SHALL 因 `json:"-"` 不出现在任何 API 响应。

贴合度计算 SHALL 在日报生成管线 Step6（section 装配、section 标题 embedding 就绪）之后、Step7（MergeSimilarSections）之前同步完成：批量 embed 当批 thread 标题，与所属 section 标题 embedding 算余弦距离写入 `fit_distance`。embedding 调用失败 SHALL 非致命——该 thread 的 `embedding`/`fit_distance` 留 NULL，前端按正常 thread 渲染（不降级、不报错）。

#### Scenario: 表新增贴合度列
- **WHEN** AutoMigrate 运行
- **THEN** `daily_report_threads` 表 SHALL 包含 `embedding`（vector 类型）与 `fit_distance`（numeric 类型）两列，均可空

#### Scenario: 接口暴露 fit_distance 但不暴露 embedding
- **WHEN** 查询日报 detail/timeline 接口返回 section.threads
- **THEN** 每条 thread 的 JSON SHALL 包含 `fit_distance` 字段（有值时）
- **AND** 每条 thread 的 JSON SHALL 不包含 `embedding` 字段（因 `json:"-"`）

#### Scenario: 贴合度在 Step6 后 Step7 前同步计算
- **WHEN** 日报生成完成 Step6 section 装配
- **THEN** 系统 SHALL 在 Step7 合并之前，批量 embed thread 标题并算出每个 thread 的 `fit_distance` 写入数据库
- **AND** 落库的 thread SHALL 携带 `embedding` 与 `fit_distance`

#### Scenario: embedding 调用失败非致命
- **WHEN** thread 标题 embedding 调用失败
- **THEN** 该 thread 的 `embedding` 与 `fit_distance` SHALL 留 NULL
- **AND** 日报生成 SHALL 继续完成（不因 embedding 失败中断）

#### Scenario: 历史 thread 贴合度留空
- **WHEN** 数据库中存在本 change 落地前生成的历史 thread
- **THEN** 其 `embedding` 与 `fit_distance` SHALL 为 NULL
- **AND** 前端 SHALL 按正常 thread 渲染（不降级、不报错），无迁移、无回刷
