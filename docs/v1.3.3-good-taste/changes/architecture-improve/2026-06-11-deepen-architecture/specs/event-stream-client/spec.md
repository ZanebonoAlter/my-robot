## ADDED Requirements

### Requirement: 单例实时事件连接
系统 SHALL 提供一个 `useEventStream()` 组合函数，全局维护唯一实时事件连接，生命周期由订阅者数量自动管理。当前后端全局事件端点仅提供 `/ws` WebSocket；专用长任务端点可继续使用 SSE Adapter。

#### Scenario: 首个事件订阅
- **WHEN** 首个组件调用 `stream.on(EVENT_TYPES.TAG_COMPLETED, handler)`
- **THEN** 发起实时事件连接到后端事件端点，返回取消订阅函数

#### Scenario: 后续组件订阅
- **WHEN** 第二个组件也调用 `stream.on(...)`
- **THEN** 复用同一实时事件连接，不发起新连接

#### Scenario: 所有订阅者卸载
- **WHEN** 所有 `on()` 返回的取消订阅函数均被调用
- **THEN** 底层连接关闭，且全局连接实例被释放，后续订阅可重新创建新连接

### Requirement: 类型化事件订阅
`useEventStream()` SHALL 提供 `on<T>(type: string, handler: (data: T) => void)` 方法，支持按事件类型注册处理器。

#### Scenario: 订阅 tag_completed 事件
- **WHEN** `stream.on<TagCompletedMessage>('tag_completed', (data) => { ... })`
- **THEN** 当实时事件流中收到 `type === 'tag_completed'` 的消息时，调用对应 handler

#### Scenario: 取消订阅
- **WHEN** 调用 `on()` 返回的取消函数
- **THEN** 该 handler 不再被触发，且订阅计数减少

#### Scenario: 组件卸载清理
- **WHEN** composable 在 `onUnmounted` 中清理
- **THEN** MUST 调用所有由 `stream.on(...)` 返回的取消订阅函数，不得遗留 handler

#### Scenario: 重复订阅同一 handler
- **WHEN** 两个组件同时订阅 `tag_completed`
- **THEN** 两个 handler 均收到事件

### Requirement: 自动重连
实时事件连接断开时 SHALL 自动尝试重连，采用指数退避策略（首次 1s，最多 30s）。

#### Scenario: 连接意外断开
- **WHEN** 后端重启导致 SSE 连接断开
- **THEN** 前端自动重连，期间未送达的消息丢失但新连接上的后续消息正常分发

#### Scenario: 空闲断开后重新订阅
- **WHEN** 最后一个订阅者退订后连接关闭，随后新组件再次订阅事件
- **THEN** `useEventStream()` MUST 创建新的连接实例，不得复用 destroyed 状态的旧实例

### Requirement: 消息类型常量
所有 SSE/WS 事件类型字符串 SHALL 集中在 `utils/eventTypes.ts` 中定义为常量。

#### Scenario: 使用消息类型
- **WHEN** 组件订阅事件 `stream.on(EVENT_TYPES.TAG_COMPLETED, handler)`
- **THEN** 不再使用内联字符串字面量 `'tag_completed'`

#### Scenario: 重命名消息类型
- **WHEN** 后端将 `tag_completed` 改为 `tag.done`
- **THEN** 只需改 `eventTypes.ts` 一处常量值，所有引用自动生效

### Requirement: 禁止绕过事件流 Seam
全局实时事件消费者 SHALL 通过 `useEventStream()` 订阅，不得在 feature/component 中直接创建 WebSocket 或 EventSource 连接。

#### Scenario: TagQueuePanel 刷新队列
- **WHEN** 队列状态需要实时刷新
- **THEN** `TagQueuePanel` MUST 订阅 `useEventStream()` 暴露的事件，而不是自行 `new WebSocket('/ws')`

#### Scenario: 专用长任务 stream
- **WHEN** 某个 API 必须使用独立的专用 SSE endpoint（例如 merge-preview scan/evaluate stream）
- **THEN** 该连接 MUST 封装在对应 API module 的命名 Adapter 中，并负责显式关闭
