<p align="center"><img src="front/public/favicon.png" width="360" alt="icon"></p>

# Syntopica

*Where feeds become topics*

Syntopica 是一个 AI-powered RSS 阅读器。  
但它的目标不是让你更快刷完文章列表，而是把持续涌入的信息流整理成可以长期追踪的语义主题。

如果你订阅了大量 RSS 源，每天都会面对几百篇新闻、博客、公告和文章。  
普通 RSS 阅读器会把它们按时间排成列表，而 Syntopica 会尝试回答另一个问题：

> 我关心的这些主题，今天到底发生了什么？

例如：

- 「AI Agent」今天有哪些新进展？
- 「金价」上涨背后有哪些连续信号？
- 「中东局势」哪些事件是昨天的延续？
- 「某个产品发布会」后续反应是否还在发酵？
- 哪些文章只是噪音，哪些值得进一步阅读？

Syntopica 希望把 RSS 从“文章时间流”变成“长期主题流”。

![主界面截图](img/1.3.2-narrative/topic-lifeline.png)

---

## 为什么做 Syntopica？

很多 RSS 用户的问题不是信息太少，而是信息没有被组织。

你可能订阅了：

- 新闻媒体
- 技术博客
- 产品更新
- 行业资讯
- GitHub / Release / Blog Feed
- RSSHub 生成的各种信息源

结果是：每天打开阅读器，看到的是一长串按时间排列的文章。

这些文章里有些属于你长期关注的议题，但它们通常被打散在不同来源、不同时间、不同标题之中。

普通 RSS 阅读器擅长回答：

> 今天来了哪些新文章？

但长期关注一个议题时，你真正需要的是：

> 这个主题今天发生了什么？  
> 它和前几天有什么关系？  
> 有哪些新信号？  
> 哪些内容值得继续看？

Syntopica 就是为这个问题设计的。

---

## 核心想法

Syntopica 不把 RSS 只当作“文章列表”，而是把每篇文章理解为某些长期主题的一部分。

它会通过 AI 和语义标签，把文章归入你关心的板块，然后每天为每个板块生成叙事简报。

```text
RSS Feeds
   ↓
Articles
   ↓
AI semantic tags / embedding
   ↓
Topic sections
   ↓
Daily narrative brief
```

你不再需要从几百篇文章里手动拼线索。
Syntopica 会把信息沉淀成可以持续追踪的 topics。


## 一个典型使用场景

把 RSS 订阅文章自动归类到几个「长期话题板块」里，每个板块每天生成一份叙事简报——就像雇了个编辑，每天帮你盯几个赛道。

> **举例**：你关注金价，新建一个「金价」板块。系统不是靠关键词硬匹配"金价"两个字，而是通过 AI 提取的小标签（黄金、美联储、贵金属、汇率…）在**语义层面**筛选相关文章，每天自动生成一份叙事报告——这个板块今天发生了什么。

![板块总览](img/1.3-feather/board_overview.png)

### 1. 手动建板块，即刻生效

知道自己想要什么？直接创建板块，填个名字（比如「中东局势」），系统会根据名称+描述**自动推荐最相关的标签**给你勾选。确认后触发回填，历史文章马上归位。

![手动创建板块](img/1.3-feather/personal_board.png)
![选择构成标签](img/1.3-feather/custom_board_choose.png)

### 2. 板块升级建议 — AI 帮你"长出新板块"

不知道手动加什么？点一下「✨ 升级建议」，大模型会分析你订阅的新闻里哪些话题频繁出现，自动聚类后给出板块创建建议——就像系统在跟你说"你最近看了很多 AI 的内容，要不要建个 AI 板块？"

建议分三种：🟢 创建新板块 / 🔵 合并到已有板块 / ⚪ 跳过（太碎片化不值得）。你挑着确认，不满意的直接跳过。

![板块升级建议](img/1.3.2-narrative/llm-upgrade.png)

### 3. 智能标签推荐 — 板块不够丰满？

感觉板块内容少、不相关？打开构成标签面板，LLM 按相似度推荐更多相关小标签，你自己勾选哪些要加进来，板块覆盖范围随手调。

![管理构成标签](img/1.3.1-bugfix/tag_may_like.png)
![板块详情](img/1.3.1-bugfix/article_match_detail.png)

### 4. 每日叙事简报

每个板块每天自动生成一份叙事报告。不是冷冰冰的摘要堆砌，而是连贯的叙述——"这个板块今天发生了什么，有什么趋势"。

![每日简报](img/1.3.2-narrative/daily-report.png)

### 🤔 它和普通 RSS、搜索引擎、RAG 有什么不同？

| 工具         | 你得到什么           | 局限             |
| ---------- | --------------- | -------------- |
| 普通 RSS 阅读器 | 按时间排列的文章列表      | 信息仍然是散的，需要自己归纳 |
| 搜索引擎       | 某一刻的搜索结果        | 需要主动搜索，缺少长期积累  |
| RAG 知识库    | 对历史内容提问         | 每次都需要你提出问题     |
| 热点监控工具     | 当前流行的热点         | 不一定匹配你的长期关注主题  |
| Syntopica  | 长期语义板块 + 每日叙事简报 | 更适合持续追踪个人关注议题  |

Syntopica 更像是一个个人议题编辑器。
它每天帮你把订阅源里的内容按长期主题整理好。

## ✨ 更多功能

### 主题图谱
- **图谱可视化**：日/周双视图，事件/人物/关键词三类节点与关联边，支持权重计算与时间窗口切换
![主题图谱](img/image-topic.png)
- **特别关注标签**：按标签类型（事件/人物/关键词）支持特别关注，后续可直接筛选仅包含该标签的文章
![category](img/1.3-feather/article_preview.png)


![主题图谱文章](img/image-topic-article.png)

### 📰 订阅管理

虽然 Syntopica 的重点是主题组织，但它仍然保留了 RSS 阅读器的基础体验：

你可以像使用普通 RSS 阅读器一样添加订阅源，也可以通过 OPML 导入已有订阅。

Syntopica 不替你决定信息来源。
你仍然掌控自己的 feeds

- Feed 管理：添加、编辑、删除、手动刷新、全量刷新
- 分类管理：自定义名称、图标、颜色
- OPML 导入导出
- 可配置自动刷新间隔
- FeedBro 风格三栏布局
- 收藏、已读标记、全屏阅读
- 预览模式与 iframe 模式切换
- 上一篇/下一篇快速导航

你可以把它当作普通 RSS 阅读器使用，也可以逐步启用 AI 主题能力。

![订阅管理界面](img/image-feed.png)
![文章阅读界面](img/image-article.png)

### 🤖 智能增强
- Firecrawl 全文抓取，补全 RSS 摘要内容
- AI 内容整理，生成结构化正文
- 内容源切换：原始内容 / Firecrawl 全文 / AI 整理稿

![内容增强状态面板](img/image-improve.png)

### ⚙️ 全局配置
- **AI Provider 路由**：多模型管理，按能力（总结/正文补全/主题提取/嵌入）分配不同 Provider，支持主备与拖拽排序
![router](img/image-router.png)
- **Firecrawl 服务**：配置 API 地址、Key、抓取模式、超时与内容长度限制
![fircrawl](img/image-firecrawl.png)
- **调度器监控**：查看 AI 总结、Feed 刷新等定时任务状态，支持手动触发与间隔调整
![fircrawl](img/image-scheduler.png)
- **队列管理**：实时监控标签打标队列、Embedding 队列的任务状态与失败重试
![queue](img/image-queue.png)
- **Feed 级设置**：单独配置每个订阅源的刷新间隔、最大保留文章数、AI 摘要开关
![queue](img/image-feed-global.png)

### 📊 阅读偏好
- 自动追踪阅读行为（打开、关闭、滚动、收藏）
- 偏好分数计算，优化排序
- 阅读统计展示
![queue](img/image-prefrence.png)

## 🛠 技术栈

### Frontend
- Nuxt
- Vue
- TypeScript
- Pinia
- Tailwind CSS

### Backend
- Go
- Gin
- GORM

### Storage
- PostgreSQL

### AI
- OpenAI-compatible API
- LLM
- Embedding Model

### Optional Services
- RSSHub
- Firecrawl

## 🚀 快速开始

### 前置条件

- [Docker](https://www.docker.com/) + Docker Compose

### 一键部署（推荐）

使用 `init.sh` 脚本自动完成环境初始化、容器启动和 AI 服务配置：

```bash
- bash init.sh  （linux用这个）
- init.ps1   （windows用这个）
```

脚本会引导完成以下步骤：

1. **基础服务** — 检查 Docker，收集端口/密码配置，启动 PostgreSQL + Syntopica 容器
2. **AI 服务**（可选）— 配置 AI 连接信息（安装和模型下载参见下方「AI 模型配置指南」）：
   - **Ollama** — 连接本地 Ollama 实例
   - **llama.cpp** — 连接本地 llama.cpp 服务
   - **远程 API** — 使用 OpenAI 兼容的云端 API（如 OpenAI、DeepSeek）
   - **跳过** — 之后通过 Web UI 手动配置
3. **Firecrawl**（可选）— 全文抓取服务：
   - **自部署** — 通过 `docker-compose.firecrawl.yml` 启动本地 Firecrawl 实例
   - **云 API** — 使用 Firecrawl 云服务
   - **跳过** — RSS 摘要模式，不抓取全文

### 手动构建与部署

**1. 交叉编译后端二进制（Linux amd64）**

```bash
cd backend-go
$env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="amd64"; go build -o syntopica ./cmd/server
```

**2. 构建 Docker 镜像并推送**

```bash
docker build -t zanebonoalter/syntopica:latest .
docker push zanebonoalter/syntopica:latest
```

> 镜像构建时会自动编译前端（`pnpm generate`），最终镜像为单容器：Go 后端同时 serve API 和前端静态文件，端口 5000。

**3. 启动服务**

```bash
# 启动核心服务（PostgreSQL + Syntopica）
docker compose up -d

# 可选：启动 Firecrawl 全文抓取服务
docker compose -f docker-compose.firecrawl.yml up -d
# 可选：启动 RssHub 服务 获取rss源
docker compose -f docker-compose.rsshub.yml up -d
```

- 访问地址：`http://localhost:5000`
- PostgreSQL 数据持久化在 `./data/` 目录
- 自定义端口/密码：在 `.env` 中配置 `BACKEND_PORT`、`POSTGRES_PASSWORD` 等

### 本地开发

```bash
# 1. 启动 PostgreSQL（需要 Docker，仅启动数据库）
docker compose -f docker-compose.pg.yml up -d

# 2. 前端
cd front && pnpm install && pnpm dev    # http://localhost:3000

# 3. 后端（需先启动 PostgreSQL）
cd backend-go && go mod tidy && go run cmd/server/main.go  # http://localhost:5000
```

### 配套服务

以下服务均为可选，无需在本地开发阶段提前启动，所有配置均可通过 Web UI 完成。

| 服务 | 本地开发选项 |
|------|-------------|
| **Firecrawl**（全文抓取） | 不启动 → RSS 摘要模式（够用）；或 `docker compose -f docker-compose.firecrawl.yml up -d` 自部署；或配置云 API |
| **LLM**（AI 增强） | 推荐 [llama.cpp](https://github.com/ggml-org/llama.cpp) 本地推理（OpenAI 兼容 API）；或 Ollama；或远程 API（OpenAI / DeepSeek 等）；或不配先用，Web UI 里配 |
| **RSSHub**（RSS 源代理） | 可选，`docker compose -f docker-compose.rsshub.yml up -d`，默认 `http://localhost:1200`。在 Feed 添加页面填入 RSSHub 实例地址即可使用 |

启动后端后访问 `http://localhost:5000` → 设置 → AI Provider 中配置 LLM 端点。例如 llama.cpp 默认地址为 `http://localhost:8080/v1`，选好模型即可使用。

## 🧠 AI 模型配置指南

`init.sh` / `init.ps1` 仅配置 AI 连接信息（IP、端口、模型名），不下载二进制或模型文件。安装和模型下载请参考以下指南。

### Ollama

1. 安装：[ollama.com](https://ollama.com)
2. 拉取模型：
   ```bash
   ollama pull qwen3:8b           # 文本模型
   ollama pull nomic-embed-text   # 嵌入模型
   ```
3. 启动服务：`ollama serve`
4. 因为ollama对json的支持不是强制的，可能导致效果一般，所以不太建议用ollama

### llama.cpp

1. 下载预编译二进制：[GitHub Releases](https://github.com/ggml-org/llama.cpp/releases)
   - Windows + NVIDIA GPU：`llama-*-bin-win-cuda-*.zip`
   - Windows CPU：`llama-*-bin-win-*.zip`
   - macOS：`llama-*-bin-macos-*.zip`
   - Linux：`llama-*-bin-linux-*.zip`
2. 下载模型文件（见下方推荐表），来源：[ModelScope](https://modelscope.cn) / [HuggingFace](https://huggingface.co)
3. 启动文本服务：
   ```bash
   ./llama-server -m model/Qwen3.5-9B-UD-Q6_K_XL.gguf -c 49152 -ngl 999 --cache-type-k q8_0 --cache-type-v q8_0 --flash-attn on --port 8080 --host 0.0.0.0 --jinja --reasoning-format deepseek --chat-template-kwargs '{\"enable_thinking\":false}' -np 2 > $null 2>&1
   ```
4. 启动嵌入服务（新终端）：
   ```bash
   ./llama-server -m model/Qwen3-Embedding-4B-Q6_K.gguf -c 8192 --embeddings --host 0.0.0.0 --pooling mean --port 8081 -ngl 0 > $null 2>&1
   ```
   > `-ngl 99` 表示全部层卸载到 GPU，无 GPU 时去掉此参数。

### VRAM 推荐模型表

| GPU VRAM | 推荐文本模型 | 推荐嵌入模型 |
|----------|-------------|-------------|
| 无 GPU | Qwen3-4B-Q4 (2.5GB) CPU | Qwen3-Emb-0.6B (0.4GB) |
| 4 GB | Qwen3-4B-Q4 (2.5GB) | Qwen3-Emb-0.6B (0.4GB) |
| 6 GB | Qwen3-8B-Q4 (4.9GB) | Qwen3-Emb-0.6B (0.4GB) |
| **8 GB** | **Qwen3-8B-Q4_K_M (4.9GB) ★** | **Qwen3-Emb-4B (3.2GB)** |
| **12 GB** | **Qwen3.5-9B-Q6 (8.0GB) ★** | **Qwen3-Emb-4B (3.2GB)** |
| 16 GB | Qwen3-14B-Q4 (9.0GB) | Qwen3-Emb-4B (3.2GB) |
| 24 GB+ | Qwen3-32B-Q4 (18GB) | Qwen3-Emb-4B (3.2GB) |

> 实际占用受 KV cache 和 context length 影响，以上为模型文件大小。

### Docker 网络说明

当 Syntopica 后端运行在 Docker 容器内时，AI 服务运行在宿主机上。后端需要通过**宿主机 IP**（而非 `localhost`）访问 AI 服务。`init.sh` 会自动检测本机 IP 并作为默认值。

手动配置时请使用宿主机局域网 IP（如 `192.168.x.x`）：
- Ollama：`http://192.168.1.100:11434/v1`
- llama.cpp 文本：`http://192.168.1.100:8080/v1`
- llama.cpp 嵌入：`http://192.168.1.100:8081/v1`

## 📂 项目结构

```
Syntopica/
├── front/                              # Nuxt 4 前端（Vue 3 + TypeScript + Pinia）
├── backend-go/                         # Go + Gin 后端（GORM + PostgreSQL）
├── docs/                               # 项目文档
├── tests/                              # Python 集成测试
├── docker/                             # Docker 构建配置
├── img/                                # 截图和图片资源
├── data/                               # PostgreSQL 数据持久化（运行时生成）
├── init.sh                             # 一键部署初始化脚本
├── docker-compose.yml                  # Docker Compose（PostgreSQL + 前后端）
├── docker-compose.pg.yml               # Docker Compose（仅 PostgreSQL，本地开发用）
├── docker-compose.firecrawl.yml        # Docker Compose（Firecrawl 全文抓取，可选）
└── docker-compose.rsshub.yml           # Docker Compose（RSSHub + Redis + Browserless，可选）
```

## Roadmap

### 当前重点
优化语义板块匹配效果
改进每日简报质量
完善板块生命周期
降低 AI 配置门槛
优化本地部署体验

### 后续可能方向
更强的主题演化分析
更清晰的板块合并 / 拆分流程
更好的跨天叙事追踪
更多 RSSHub / 全文抓取适配
更完善的图谱视图
移动端体验优化

## 适合谁使用？

### Syntopica 适合这些用户：

- 每天阅读大量 RSS 的人
- 长期关注某些行业、公司、技术或市场的人
- 不想被信息流淹没，但又不想放弃信息密度的人
- 希望用 AI 辅助整理信息，而不是只做单篇摘要的人
- 想搭建个人信息雷达的人

### 它不太适合：

- 只想偶尔看几篇新闻的人
- 不需要长期主题追踪的人
- 完全不想配置 AI 服务的人
- 只需要极简 RSS 客户端的人

## 🤝 项目状态

Syntopica 仍在持续开发中。

当前更偏个人使用和实验性产品阶段。
部分能力仍在调整，包括语义匹配、板块推荐、简报生成和本地模型适配。

如果你对 RSS、个人信息管理、AI 内容组织、主题追踪感兴趣，欢迎试用、反馈或参与改进。

## License

[GNU General Public License v3.0](LICENSE)
