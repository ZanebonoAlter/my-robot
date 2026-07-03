# ocr（open-code-review）调用避坑与 token 纪律

> 活文档。踩到一次就补一条，避免重复交学费。
> 适用：openspec `开发执行规范.md` §0.6 步骤4（code review）调用 ocr 时。

## 为什么有这份

ocr（`@alibaba-group/open-code-review`）做 AI 代码审查，**单次 review = N 个文件 × 并发 LLM 调用**，很贵。一次 topic-watchlist-observability change 的 review 实测：21 文件并发 × glm-5.2，一次就烧一批 token；为"格式化输出"重跑又撞限流空烧。本文件把调用前必须确认的事固化为纪律。

## 核心教训（按踩坑顺序）

### 1. text 输出首次成功即为完整结果，不要为"格式"重跑

**现象**：`ocr review --audience agent`（默认 text 格式）成功返回，但 stdout 被 `tail` 截断显示，看起来"没拿全"。于是改 `--format json` 重跑。

**根因**：显示截断（`tail -80`）≠ 数据缺失。ocr 的 text 输出在 stdout 退出时就是完整的，只是终端没全显示。

**正确做法**：
- text 首次成功就视为拿到全部结果。
- 想完整看，**重定向到文件**：`ocr review ... > /tmp/ocr.txt 2>&1`，再 `read` / `cat` 读全文。**绝不为"漂亮的 json 分类"重跑一次 review**。
- 要 json 格式，首次就该指定 `--format json`，别 text 跑完再 json 重跑。

### 2. 限流是常态，重跑会空烧 token

**现象**：第二次 `ocr review` 报 `all 21 file review(s) failed — check LLM configuration`，exit 1，输出 0 字节。但第一次明明成功。

**根因**：glm-5.2 等国内模型 rate limit 严，21 文件并发（默认 `--concurrency 8`）极易触发；限流后整批失败，LLM token 已烧（请求发出去了）。

**正确做法**：
- **一次成功就够**，限流后不要立即原样重跑。
- 降低并发：`--concurrency 3`（慢但稳）。
- 范围大时优先隔离脏改动（见第 3 条）缩小文件数，而不是硬扛并发。

### 3. 脏工作区会被卷入，先隔离再 review

**现象**：`git status` 100+ 文件 modified（多数是 CRLF 假改动），`ocr review` 默认 review staged + unstaged + untracked 全部，preview 列出几十个无关文件（`docs/v1.3.4/`、接手前的 `front/app/utils/*` 等），既爆 token 又污染 review 结论。

**正确做法**（按优先级）：
1. **首选 commit 对比**：本次改动若已 commit，用 `ocr review --from <parent> --to HEAD`，只 review commit diff，完全绕开工作区脏。
2. **次选 stash 隔离**：改动还在工作区时，`git add` 本次文件 → `git stash push --keep-index` 把噪声 stash 走 → review staged + untracked → review 完 `git stash pop`。**注意 pop 可能冲突**，用完确认 stash 已清理（`git stash list`）。
3. **preview 先行**：跑前先 `ocr review --preview`，看 `default_path` 标记的文件数（= 真会进 LLM 的），超预期就先隔离。

### 4. json 解析不要在 bash 内联 python

**现象**：`ocr review --format json | python3 -c "..."`，bash 单引号包 f-string + 字典 `.get()` 嵌套引号，转义地狱，syntax error。

**正确做法**：
- `ocr review --format json > /tmp/ocr.json 2>/tmp/ocr.err` 存盘。
- 用 `read` 工具或 ctx_execute 沙箱（python/node）解析文件，引号环境干净。

### 5. ocr 的 text 输出已含 suggestion（diff 片段），够用于步骤4

**现象**：以为 text 格式只有 comment 文字、没有代码建议，非要 json 拿 `suggestion_code`。

**根因**：text 格式其实**完整渲染了 existing_code/suggestion_code 的 diff 块**（红绿行），comment + 定位行号 + 建议代码都有。json 只是结构化，信息量相同。

**正确做法**：步骤4（code review）用 text 输出完全够。只在需要程序化后处理（如自动 apply fix）时才用 json。

## 调用 ocr 前的自检清单

- [ ] 先 `ocr review --preview`，确认 `default_path` 文件数 = 预期范围（噪声已隔离）
- [ ] text 格式优先（步骤4 够用），json 仅在需要程序化处理时
- [ ] 输出重定向到文件（`> /tmp/ocr.txt`），别依赖终端 tail
- [ ] `--concurrency` 调到 3-5（限流场景）
- [ ] 业务上下文必给（`-b "..."`），提升 review 命中率、减少无意义 comment
- [ ] 限流失败后：不立即原样重跑；缩小范围或降并发再试

## 反例（本次踩的）

```
✗ ocr review --audience agent ... | tail -80        # text 成功但显示截断，误判"没拿全"
✗ ocr review --format json | python3 -c "..."       # bash 引号地狱，syntax error
✗ ocr review --format json > file                   # 第三次重跑，限流，21 file 全失败空烧 token
```

正解应该是：
```
✓ ocr review --audience agent -b "..." > /tmp/ocr.txt 2>&1   # 一次，text，存盘
✓ read /tmp/ocr.txt                                          # 完整读，分类汇报
```
