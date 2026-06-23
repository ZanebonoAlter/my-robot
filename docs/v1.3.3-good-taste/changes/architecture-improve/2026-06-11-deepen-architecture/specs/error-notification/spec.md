## ADDED Requirements

### Requirement: 全局通知管道
系统 SHALL 提供一个 `useNotify()` 组合函数，暴露 `success(msg)`、`error(msg)`、`warn(msg)` 三个方法向全局 toast 队列推送消息。Toast 队列 SHALL 使用 Nuxt `useState('notify:toasts')` 管理，不得使用模块级 `ref` 保存全局可变状态。

#### Scenario: Store/Composable 写操作失败
- **WHEN** 执行业务写操作的 Store 或 Composable 收到 API 失败响应
- **THEN** 由该 Store/Composable 调用 `notify.error(...)`，底层 API module 不直接弹 toast

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

#### Scenario: SSR/多实例安全
- **WHEN** Nuxt 在服务端或多请求环境创建应用实例
- **THEN** toast 队列按 Nuxt state 隔离，不通过模块级 `ref` 跨请求共享

### Requirement: 避免重复错误通知
同一次失败 SHALL 只有一个责任层推送 toast，避免 API store、feature store、view 同时弹出重复通知。

#### Scenario: 文章收藏失败
- **WHEN** `useArticlesStore().toggleFavorite()` API 调用失败
- **THEN** 只由 `useArticlesStore` 推送一次错误通知，调用方组件不再重复 `notify.error(...)`

### Requirement: 组件不再持有独立全局错误 ref
View 组件 SHALL 不再维护独立的 `error` / `notice` ref 用于全局错误反馈，统一通过 `useNotify()` 或执行操作的 Store/Composable 推送。局部表单校验或 inline error 可保留在组件内部。

#### Scenario: TopicGraphPage 加载失败
- **WHEN** `loadHotspots()` 调用失败
- **THEN** 调用 `notify.error('图谱加载失败')`，不再设置 `notice.value = ...` 并在模板中条件渲染
