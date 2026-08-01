## MODIFIED Requirements

### Requirement: 长锁 DDL 须在 lock_timeout 守卫内执行

涉及 `ALTER COLUMN ... TYPE`（全表重写）、`ADD CONSTRAINT ... UNIQUE`（全表扫描）或 `ADD CONSTRAINT ... FOREIGN KEY`（全表行校验）等长 `AccessExclusiveLock` 的迁移 DDL SHALL 通过 `withLockTimeout(db, timeout, fn)` 守卫执行。该 helper SHALL 在事务内 `SET LOCAL lock_timeout = <timeout>` 后执行 `fn`，并在 `fn` 返回后 `SET LOCAL lock_timeout = DEFAULT` 显式复位。当锁竞争使语句在 `timeout`（默认 5s）内无法获取锁时，该语句 SHALL 失败（而非无限阻塞），以保证大表写入不被无限期锁住。

#### Scenario: withLockTimeout 在事务内设置 GUC 并于执行后复位

- **WHEN** 在事务内调用 `withLockTimeout(db, "3s", fn)`
- **THEN** `fn` 执行期间 `SHOW lock_timeout` 返回 `3s`（GUC 生效）
- **AND** `fn` 返回后同事务内 `SHOW lock_timeout` 回到 `DEFAULT`（显式复位）

#### Scenario: 长锁 DDL 守卫点覆盖四类高危操作

- **WHEN** 迁移含 `ALTER COLUMN embedding TYPE vector(N)`、`ADD CONSTRAINT ... UNIQUE`、扩展既有 `UNIQUE` 约束、或 `ADD CONSTRAINT ... FOREIGN KEY`
- **THEN** 这些语句各自被 `withLockTimeout(db, "5s", ...)` 守卫包裹
- **AND** 守卫在成功路径上不影响迁移结果（仅在锁竞争超 5s 时让该语句失败而非无限阻塞）

#### Scenario: ADD CONSTRAINT FOREIGN KEY 迁移须在 lock_timeout 守卫内执行

- **WHEN** 一条版本迁移含 `ALTER TABLE topic_watch_hits ADD CONSTRAINT fk_topic_watch_hits_watch FOREIGN KEY (watch_id) REFERENCES board_topic_watches(id) ON DELETE CASCADE`（如 `20260801_0002`）
- **THEN** 该 ADD CONSTRAINT 语句被 `withLockTimeout(db, "5s", fn)` 包裹
- **AND** FK 校验扫描期间持有 `AccessExclusiveLock`，锁竞争超 5s 时语句失败而非无限阻塞
