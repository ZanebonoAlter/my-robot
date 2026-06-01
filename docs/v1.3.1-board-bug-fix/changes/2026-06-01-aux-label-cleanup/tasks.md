## 1. Phase 1: ref_count 修正

- [x] 在 `AuxiliaryLabelService` 新增 `RecountRefs(ctx, ids []uint) error` 方法
- [x] 在 `CleanupOrphanedTags`（article_tagger.go）中，删除 topic_tag 前收集 aux label IDs，删除后调用 `RecountRefs` 重算
- [x] 在 `HardMergeTags`（hard_merge.go）中，删除 source tag 前收集 aux label IDs，删除后调用 `RecountRefs` 重算
- [ ] 1.4 编写 `RecountRefs` 和 `CleanupOrphanedTags` 修正逻辑的单元测试

## 2. Phase 2: GC 服务

- [x] 在 `AuxiliaryLabelService` 新增 `GC(ctx, req AuxLabelGCRequest) (*AuxLabelGCResult, error)` 方法，支持 dry_run / disable / delete / recalculate 四种模式
- [x] GC disable 模式同时清理 `board_composition` 中引用被 disable aux label 的行
- [x] 在 `semanticBoardHandler` 新增 `POST /api/auxiliary-labels/gc` 端点
- [ ] 2.4 编写 GC 逻辑的单元测试

## 3. Phase 3: 定时任务

- [x] 新建 `backend-go/internal/jobs/aux_label_cleanup.go`，实现 `AuxLabelCleanupScheduler`（仿照 LogCleanupScheduler）
- [x] 在 `runtimeinfo/schedulers.go` 声明 `AuxLabelCleanupSchedulerInterface`
- [x] 在 `handler.go` schedulerDescriptors 中注册 `aux_label_cleanup`
- [x] 在 `runtime.go` 中初始化并启动 `AuxLabelCleanupScheduler`，注册到 graceful shutdown
- [ ] 3.5 编写调度器单元测试

## 4. 前端集成

- [x] 在 `front/app/utils/schedulerMeta.ts` 添加 `aux_label_cleanup` 的 displayName/icon/color
- [x] 在 `front/app/api/auxiliaryLabels.ts` 添加 `triggerGc` API 方法
- [x] 前端 lint + typecheck 验证

## 5. 验证

- [x] 运行 `golangci-lint run ./...` 和 `go vet ./...`
- [ ] 5.2 运行 `go test ./internal/domain/tagging/... ./internal/jobs/...`
- [x] 运行 `go build ./...`
- [x] 前端 `pnpm lint` + `pnpm exec nuxi typecheck` + `pnpm build`
