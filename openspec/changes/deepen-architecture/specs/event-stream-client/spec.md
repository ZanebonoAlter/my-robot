## ADDED Requirements

### Requirement: 单例 SSE 连接
系统 SHALL 提供一个 `useEventStream()` 组合函数，全局维护唯一 SSE 连接，生命周期由函数自动管理。

#### Scenario: 首个组件调用
- **WHEN** 首个组件 `const stream = useEventStream()`
- **THEN** 发起 SSE 连接到后端事件端点，返回连接实例

#### Scenario: 后续组件调用
- **WHEN** 第二个组件也调用 `useEventStream()`
- **THEN** 返回同一连接实例，不发起新连接

#### Scenario: 所有订阅者卸载
- **WHEN** 所有使用 `useEventStream()` 的组件 `onUnmounted`
- **THEN** 底层 SSE 连接关闭

### Requirement: 类型化事件订阅
`useEventStream()` SHALL 提供 `on<T>(type: string, handler: (data: T) => void)` 方法，支持按事件类型注册处理器。

#### Scenario: 订阅 tag_completed 事件
- **WHEN** `stream.on<TagCompletedMessage>('tag_completed', (data) => { ... })`
- **THEN** 当 SSE 流中收到 `type === 'tag_completed'` 的消息时，调用对应 handler

#### Scenario: 取消订阅
- **WHEN** 调用 `on()` 返回的取消函数
- **THEN** 该 handler 不再被触发

#### Scenario: 重复订阅同一 handler
- **WHEN** 两个组件同时订阅 `tag_completed`
- **THEN** 两个 handler 均收到事件

### Requirement: 自动重连
SSE 连接断开时 SHALL 自动尝试重连，采用指数退避策略（首次 1s，最多 30s）。

#### Scenario: 连接意外断开
- **WHEN** 后端重启导致 SSE 连接断开
- **THEN** 前端自动重连，期间未送达的消息丢失但新连接上的后续消息正常分发

### Requirement: 消息类型常量
所有 SSE/WSS 事件类型字符串 SHALL 集中在 `utils/eventTypes.ts` 中定义为常量。

#### Scenario: 使用消息类型
- **WHEN** 组件订阅事件 `stream.on(EVENT_TYPES.TAG_COMPLETED, handler)`
- **THEN** 不再使用内联字符串字面量 `'tag_completed'`

#### Scenario: 重命名消息类型
- **WHEN** 后端将 `tag_completed` 改为 `tag.done`
- **THEN** 只需改 `eventTypes.ts` 一处常量值，所有引用自动生效
