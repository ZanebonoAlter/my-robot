## ADDED Requirements

### Requirement: 全局通知管道
系统 SHALL 提供一个 `useNotify()` 组合函数，暴露 `success(msg)`、`error(msg)`、`warn(msg)` 三个方法向全局 toast 队列推送消息。

#### Scenario: API 调用失败
- **WHEN** API store 中 `fetchArticles()` 返回失败
- **THEN** 自动调用 `notify.error('文章加载失败')` 而非设置 `error.value`

#### Scenario: 组件主动通知
- **WHEN** 组件调用 `notify.success('已保存')`
- **THEN** 页面顶部显示绿色 toast，3 秒后自动消失

### Requirement: Toast 容器组件
系统 SHALL 在 `app.vue` 层级渲染一个 `<NotifyContainer>` 组件，监听通知队列并渲染 toast 列表。

#### Scenario: 多条通知排队
- **WHEN** 3 条通知同时在 200ms 内产生
- **THEN** 按时间顺序叠放显示，每条各自计时消失

#### Scenario: 用户手动关闭
- **WHEN** 用户点击 toast 的关闭按钮
- **THEN** 该 toast 立即移除，不影响其他 toast

### Requirement: 组件不再持有独立 error ref
View 组件 SHALL 不再维护独立的 `error` / `notice` ref 用于错误展示，统一通过 `useNotify()` 或上级（API store）推送。

#### Scenario: TopicGraphPage 加载失败
- **WHEN** `loadHotspots()` 调用失败
- **THEN** 调用 `notify.error('图谱加载失败')`，不再设置 `notice.value = ...` 并在模板中条件渲染
