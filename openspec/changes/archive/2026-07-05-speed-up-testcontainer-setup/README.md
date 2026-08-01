# speed-up-testcontainer-setup

加速后端集成测试：`topicgraph/repository` 包 41 个 `SetupTestDB` 调用点每次都重建 schema（DROP+rebuild，容器本身已是单例），耗时 ~382s。改为进程级一次性「黄金 schema」+ 测试间 `ResetTestData`（truncate + 恢复 seed + 恢复 vector 列维度 + 重开连接池），实测降至 ~147s（~2.6x）。`ReimportTestDB` 作为逃生口保留。
