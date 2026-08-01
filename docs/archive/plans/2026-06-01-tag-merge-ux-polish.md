# Tag Merge UX Polish 实施计划

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** 修复 SSE 竞态、添加页面恢复、"按 AI 建议全选"按钮、合并进度展示

**Architecture:** 前后端协同。后端 SSE handler 阻塞等待 channel + 新增 status 端点；前端先建 EventSource 再 POST 触发 + 调 status 恢复连接 + UI 按钮和进度。

**Tech Stack:** Go (Gin), Vue 3 + TypeScript, SSE (EventSource)

---

## Task 1: 后端 SSE handler 阻塞等待 channel

**Files:**
- Modify: `backend-go/internal/domain/tagging/tag_merge_preview_handler.go` — `EvaluateStreamHandler`, `ScanStreamHandler`

**Step 1: 修改 EvaluateStreamHandler**

当前逻辑（line ~340-365）：channel==nil 时发 idle 立即返回。改为阻塞等待：

```go
func EvaluateStreamHandler(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    // 等待 channel 出现（前端先连 SSE，再 POST 触发）
    ch := waitForEvaluateChannel(c.Request.Context())
    if ch == nil {
        c.SSEvent("progress", EvaluateProgress{Status: "idle"})
        return
    }

    c.Stream(func(w io.Writer) bool {
        msg, ok := <-ch
        if !ok {
            return false
        }
        c.SSEvent("progress", msg)
        return true
    })
}
```

**Step 2: 添加 waitForEvaluateChannel 辅助函数**

放在 `tag_merge_suggest.go` 中（和 `GetEvaluateProgressChannel` 同文件）：

```go
// WaitForEvaluateChannel 轮询等待 eval channel 创建，超时返回 nil
func WaitForEvaluateChannel(ctx context.Context) <-chan EvaluateProgress {
    const maxWait = 30 * time.Second
    const pollInterval = 200 * time.Millisecond
    deadline := time.Now().Add(maxWait)
    for time.Now().Before(deadline) {
        select {
        case <-ctx.Done():
            return nil
        default:
        }
        if ch := GetEvaluateProgressChannel(); ch != nil {
            return ch
        }
        time.Sleep(pollInterval)
    }
    return nil
}
```

同样添加 `WaitForScanChannel`，结构相同，调用 `GetScanProgressChannel`。

**Step 3: 修改 ScanStreamHandler 同样使用 waitFor 模式**

```go
func ScanStreamHandler(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    ch := WaitForScanChannel(c.Request.Context())
    if ch == nil {
        c.SSEvent("progress", ScanProgress{Status: "idle"})
        return
    }

    c.Stream(func(w io.Writer) bool {
        msg, ok := <-ch
        if !ok {
            return false
        }
        c.SSEvent("progress", msg)
        return true
    })
}
```

**Step 4: Handler 中直接调用**

在 `tag_merge_preview_handler.go` 中，handler 直接调用 `tagging.WaitForEvaluateChannel(c.Request.Context())` 和 `tagging.WaitForScanChannel(c.Request.Context())`。

**Step 5: 编译验证**

Run: `cd backend-go && go build ./...`
Expected: 编译通过

**Step 6: Commit**

```bash
git add backend-go/internal/domain/tagging/tag_merge_suggest.go backend-go/internal/domain/tagging/tag_merge_preview_handler.go
git commit -m "fix(backend): SSE handlers wait for channel instead of immediate close on nil"
```

---

## Task 2: 前端 triggerEvaluate() — 先 SSE 后 POST

**Files:**
- Modify: `front/app/features/topic-graph/components/TagMergePreview.vue` — `triggerEvaluate()` 函数

**Step 1: 修改 triggerEvaluate()**

当前逻辑：先 POST → 再创建 EventSource。改为先 EventSource → 再 POST。

找到 `triggerEvaluate()` 函数（约 line 200-230），改为：

```ts
async function triggerEvaluate() {
  evaluating.value = true
  evalProgress.value = null

  // 先建立 SSE 连接，再 POST 触发（避免竞态）
  evalEs = api.createEvaluateEventSource((progress) => {
    evalProgress.value = progress
    if (progress.status === 'done' || progress.status === 'error') {
      evalEs?.close()
      evalEs = null
      evaluating.value = false
      if (progress.status === 'done') {
        loadGroups()
      }
    }
  })

  try {
    await api.triggerEvaluate()
  } catch (err: any) {
    // 409 = 已有评估运行中，SSE 连接可以继续接收进度
    if (err?.statusCode === 409) {
      // 不关 SSE，继续接收已有进度
      return
    }
    // 其他错误：关闭 SSE，重置状态
    evalEs?.close()
    evalEs = null
    evaluating.value = false
    console.error('触发评估失败:', err)
  }
}
```

**Step 2: 修改 cancelEvaluate() 确保清理**

确认 `cancelEvaluate()` 正确关闭 EventSource 和重置状态。现有逻辑应该已经 OK，但检查下。

**Step 3: Commit**

```bash
git add front/app/features/topic-graph/components/TagMergePreview.vue
git commit -m "fix(frontend): triggerEvaluate opens SSE before POST to avoid race condition"
```

---

## Task 3: 前端 triggerFullScan() — 先 SSE 后 POST

**Files:**
- Modify: `front/app/features/topic-graph/components/TagMergePreview.vue` — `triggerFullScan()` 函数

**Step 1: 修改 triggerFullScan()**

与 Task 2 完全相同的模式。找到 `triggerFullScan()` 函数，改为先 EventSource → 再 POST：

```ts
async function triggerFullScan() {
  scanning.value = true
  scanProgress.value = null

  // 先建立 SSE 连接，再 POST 触发
  scanEs = api.createScanEventSource((progress) => {
    scanProgress.value = progress
    if (progress.status === 'done' || progress.status === 'error') {
      scanEs?.close()
      scanEs = null
      scanning.value = false
      if (progress.status === 'done') {
        loadGroups()
      }
    }
  })

  try {
    await api.triggerFullScan()
  } catch (err: any) {
    if (err?.statusCode === 409) {
      return
    }
    scanEs?.close()
    scanEs = null
    scanning.value = false
    console.error('触发扫描失败:', err)
  }
}
```

**Step 2: Commit**

```bash
git add front/app/features/topic-graph/components/TagMergePreview.vue
git commit -m "fix(frontend): triggerFullScan opens SSE before POST to avoid race condition"
```

---

## Task 4: 后端新增 status 端点

**Files:**
- Modify: `backend-go/internal/domain/tagging/tag_merge_suggest.go` — 新增 `GetScanRunning()`, `GetEvalRunning()` 导出方法
- Modify: `backend-go/internal/domain/tagging/tag_merge_preview_handler.go` — 新增 `MergePreviewStatusHandler`
- Modify: `backend-go/internal/domain/tagging/tag_merge_preview_handler.go` — 路由注册

**Step 1: 在 tag_merge_suggest.go 添加运行状态查询**

```go
// GetScanRunning 返回扫描是否正在运行
func GetScanRunning() bool {
    return scanState.running.Load()
}

// GetEvalRunning 返回评估是否正在运行
func GetEvalRunning() bool {
    return evalState.running.Load()
}
```

**Step 2: 新增 Status handler**

在 `tag_merge_preview_handler.go` 添加：

```go
func MergePreviewStatusHandler(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "scan_running": tagging.GetScanRunning(),
        "eval_running": tagging.GetEvalRunning(),
    })
}
```

**Step 3: 注册路由**

在 `RegisterTagMergePreviewRoutes` 中添加：

```go
mergePreview.GET("/status", MergePreviewStatusHandler)
```

**Step 4: 编译验证**

Run: `cd backend-go && go build ./...`
Expected: 编译通过

**Step 5: Commit**

```bash
git add backend-go/internal/domain/tagging/tag_merge_suggest.go backend-go/internal/domain/tagging/tag_merge_preview_handler.go
git commit -m "feat(backend): add GET /merge-preview/status endpoint for page recovery"
```

---

## Task 5: 前端 API client 添加 getStatus 方法

**Files:**
- Modify: `front/app/api/tagMergePreview.ts` — 新增 `getMergePreviewStatus()`

**Step 1: 添加 API 方法**

```ts
async getMergePreviewStatus(): Promise<{ scan_running: boolean; eval_running: boolean }> {
    return this.client.get('/topic-tags/merge-preview/status')
}
```

**Step 2: Commit**

```bash
git add front/app/api/tagMergePreview.ts
git commit -m "feat(frontend): add getMergePreviewStatus API method"
```

---

## Task 6: 前端页面恢复 SSE

**Files:**
- Modify: `front/app/features/topic-graph/components/TagMergePreview.vue` — `loadGroups()` 或 `onMounted`/`watch`

**Step 1: 添加 recoverRunningState 函数**

```ts
async function recoverRunningState() {
  try {
    const status = await api.getMergePreviewStatus()
    if (status.eval_running) {
      evaluating.value = true
      evalEs = api.createEvaluateEventSource((progress) => {
        evalProgress.value = progress
        if (progress.status === 'done' || progress.status === 'error') {
          evalEs?.close()
          evalEs = null
          evaluating.value = false
          if (progress.status === 'done') {
            loadGroups()
          }
        }
      })
    }
    if (status.scan_running) {
      scanning.value = true
      scanEs = api.createScanEventSource((progress) => {
        scanProgress.value = progress
        if (progress.status === 'done' || progress.status === 'error') {
          scanEs?.close()
          scanEs = null
          scanning.value = false
          if (progress.status === 'done') {
            loadGroups()
          }
        }
      })
    }
  } catch (err) {
    console.error('查询合并预览状态失败:', err)
  }
}
```

**Step 2: 在 loadGroups 前调用**

修改 `loadGroups()` 或在组件初始化逻辑中，先调 `recoverRunningState()`。如果正在运行，进度条自动显示；加载分组可以并行进行（已完成的分组仍可显示）。

在 `watch(visible)` 或 `onMounted` 中：
```ts
watch(visible, (v) => {
  if (v) {
    recoverRunningState()
    loadGroups()
  }
})
```

如果 standalone 模式（无 visible prop），在 `onMounted` 中调用。

**Step 3: Commit**

```bash
git add front/app/features/topic-graph/components/TagMergePreview.vue
git commit -m "feat(frontend): auto-recover SSE connections on page re-entry"
```

---

## Task 7: "按 AI 建议全选"按钮

**Files:**
- Modify: `front/app/features/topic-graph/components/TagMergePreview.vue` — template + 可选 script

**Step 1: 在 header 区域添加按钮**

在模板中找到已有的 header/toolbar 区域，添加"按 AI 建议全选"按钮。放在"合并选中"按钮旁边：

```html
<button
  class="px-3 py-1.5 text-sm rounded-md border transition-colors"
  :class="hasMergeableSuggestions
    ? 'bg-blue-50 text-blue-700 border-blue-300 hover:bg-blue-100 dark:bg-blue-900/30 dark:text-blue-300 dark:border-blue-700 dark:hover:bg-blue-900/50'
    : 'bg-gray-50 text-gray-400 border-gray-200 cursor-not-allowed dark:bg-gray-800 dark:text-gray-600 dark:border-gray-700'"
  :disabled="!hasMergeableSuggestions"
  @click="selectAllMergeable()"
>
  按 AI 建议全选
</button>
```

**Step 2: 添加 computed**

```ts
const hasMergeableSuggestions = computed(() => {
  return groups.value?.some(g =>
    g.suggestions?.some(s => {
      const verdict = parseLLMVerdict(s.llm_verdict)
      return !verdict || verdict.should_merge
    })
  ) ?? false
})
```

注意：`selectAllMergeable()` 已经存在，不需要重写。

**Step 3: 验证**

Run: `cd front && pnpm lint`
Expected: lint 通过

**Step 4: Commit**

```bash
git add front/app/features/topic-graph/components/TagMergePreview.vue
git commit -m "feat(frontend): add 'select all AI suggestions' button to merge preview"
```

---

## Task 8: 合并进度展示

**Files:**
- Modify: `front/app/features/topic-graph/components/TagMergePreview.vue` — `mergeSelected()` + template

**Step 1: 添加进度 ref**

```ts
const mergeProgress = ref<{ done: number; total: number } | null>(null)
```

**Step 2: 修改 mergeSelected()**

在函数开头设置进度，循环中更新：

```ts
async function mergeSelected() {
  // ... 现有的 selectedKeys 遍历逻辑
  const keys = Array.from(selectedKeys.value)
  mergeProgress.value = { done: 0, total: keys.length }
  
  for (const key of keys) {
    // ... 现有的单个合并逻辑
    mergeProgress.value!.done++
  }
  
  mergeProgress.value = null
  // ... 现有的 reload 逻辑
}
```

**Step 3: 在 template 中显示进度**

在"合并选中"按钮附近或进度条区域：

```html
<span v-if="mergeProgress" class="text-sm text-gray-500 dark:text-gray-400">
  {{ mergeProgress.done }}/{{ mergeProgress.total }} 已合并
</span>
```

**Step 4: 验证**

Run: `cd front && pnpm lint`
Expected: lint 通过

**Step 5: Commit**

```bash
git add front/app/features/topic-graph/components/TagMergePreview.vue
git commit -m "feat(frontend): show merge progress N/M during batch merge"
```

---

## 任务依赖关系

```
Task 1 (后端 SSE 阻塞) ─────┐
Task 2 (前端 evaluate SSE) ──┤──→ 可并行
Task 3 (前端 scan SSE) ──────┘
         ↓
Task 4 (后端 status 端点) ──→ Task 5 (前端 API) ──→ Task 6 (前端恢复)
Task 7 (全选按钮) ──→ 独立，可并行
Task 8 (合并进度) ──→ 独立，可并行
```

## 可并行的组

**组 A（SSE 修复，Tasks 1+2+3）**：前后端配合修复竞态，顺序执行
**组 B（页面恢复，Tasks 4+5+6）**：依赖组 A 完成
**组 C（全选按钮，Task 7）**：完全独立
**组 D（合并进度，Task 8）**：完全独立

**推荐执行顺序**：组 A → 组 B，组 C 和组 D 可在任意时间并行。
