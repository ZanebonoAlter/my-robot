# speed-up-testcontainer-setup

加速后端集成测试：当前每个 SetupTestDB 调用都重起 pgvector testcontainer，41 个调用点导致 repository 包测试耗时 ~355s。目标：单例容器 + ReimportTestDB 复用，将反馈时间从 ~6 分钟降到 ~1 分钟。
