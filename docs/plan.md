# 软件商店系统规划

版本：v0.1.0  
范围：LazyCat LPK 私有/自托管软件商店，服务端与客户端为两个可独立部署的裸应用。

## 格局判断

### Thesis

这个项目不应该做成“一个带上传页面的软件源”，而应该做成“可信发布控制面 + 多源消费客户端”。服务端负责治理、权限、审核、存储与对外源协议，并内嵌自己的 Web 控制台；客户端只负责订阅、浏览、安装和用户互动，可在不部署服务端时独立使用。

### Confidence

- **Confidence level**: high
- **Why not certain**: LazyCat 官方 SDK 的实际运行只能在 LazyCat WebShell 环境内完整验证，本地浏览器只能验证降级路径。

### The Trap

- **Inherited constraint**: 为了快速上线，把服务端管理台、客户端商店和软件源协议揉成一个应用。
- **Is it real?**: no
- **Why**: 需求已经明确客户端和服务端是两个应用程序；软件源订阅也是面向外部客户端的公共契约，不能被内部 UI 形态绑死。

### High-格局 Direction

系统拆成三层：

1. **发布治理层**：用户、角色、审核、版本、协作者、群组可见性、评论、过期标记、API Token。
2. **分发协议层**：统一软件源端点、密码保护、密码轮转、镜像改写、SHA256 校验字段。
3. **消费体验层**：多源订阅、分类/搜索/详情/安装、收藏、反馈、LazyCat SDK 安装。

### Frame-Opening Move

- **Move used**: end-state backcasting
- **What it reveals**: 六个月后的优秀形态不是“一个站点能上传 LPK”，而是任何 CI、维护者、管理员和终端用户都可以围绕同一套发布事实工作。

### Bold Takes

- 服务端必须拥有最终发布状态，客户端不能绕过审核或私有可见性。
- LPK SHA256 是安装链路的硬约束；没有 checksum 的外部链接不能进入正式版本。
- GitHub 镜像是下载策略，不是版本事实；源协议要保留 upstream URL，让不同客户端能配置自己的镜像。
- SQLite 是默认单机体验，PostgreSQL/MySQL 是正式部署路径，三种数据库必须走同一套 ent 模型。

### Options

| Option | What it optimizes | Cost | Verdict |
|---|---|---|---|
| Conservative path | 快速做一个上传/下载页面 | 审核、权限、CI/CD、多源客户端后期都会返工 | reject |
| Clean target | 从第一天建立发布控制面、源协议、独立客户端 | 初始模型和 API 面较大 | recommended |
| Staged clean path | 先实现完整闭环，再补真实对象存储和高级治理 | 需要持续验收清单 | fallback |

### What Not To Do

- 不把管理员功能隐藏在客户端本地状态里。
- 不允许客户端安装时跳过 SHA256 校验。
- 不为每种数据库写分叉业务逻辑。
- 不把 GitHub 镜像后的 URL 写回版本事实。

### First Proof Point

本地用 SQLite 启动服务端和客户端，管理员创建分类、普通用户提交 LPK 或外部版本、管理员审批、源端点暴露版本、客户端通过 LazyCat SDK 或校验后下载完成安装动作。

### Falsifier

如果 LazyCat SDK 要求安装必须走官方仓库签名或固定源格式，当前源协议就需要增加签名/元数据字段，客户端安装链路也要相应调整。

### Payoff Ledger（收益账单）

| Move | Price paid now | What it buys | When the payoff shows |
|---|---|---|---|
| 拆分服务端和客户端 | 两套构建、两套部署配置 | 软件源可以被官方客户端、社区客户端、CI 同时消费 | 第一次接入第二个软件源消费者时 |
| 强制版本 SHA256 | 上传/外链流程多一个字段或计算步骤 | 安装失败能明确中止，不会静默安装被篡改的 LPK | 第一次外部链接失效或被替换时 |
| 用 ent 统一三种数据库 | Schema 设计需要更克制 | SQLite 单机和 PostgreSQL/MySQL 生产部署不分叉 | 从本地迁到服务器时 |
| 源协议保留 upstream URL | Feed 字段略多 | 客户端可以按自己的网络环境配置 GitHub 镜像 | 第一次用户换镜像源时 |

## 应用边界

| 应用 | 路径 | 职责 | 不负责 |
|---|---|---|---|
| 服务端 | `cmd/store-server` + `lazycat/server` | REST API、源端点、审核、权限、存储、数据库迁移、嵌入式 Web 控制台 | 终端安装 UI |
| 客户端 | `client` + `lazycat/client` | 多源订阅、浏览、搜索、详情、安装、收藏、反馈；可独立部署 | 最终审核状态和权限裁决 |

## 服务端规划

### 技术栈

- Go `net/http`，通过 `embed.FS` 内嵌 Vite 构建产物
- ent ORM
- SQLite3 / PostgreSQL / MySQL，通过 `DB_DRIVER` 和 `DB_DSN` 切换
- 本地文件、WebDAV、S3、GitHub 外链存储模式
- OpenAPI 文档：`docs/openapi.yaml`

### 核心模块

| 模块 | 主要能力 |
|---|---|
| Auth | 注册、登录、邮箱验证、Session Cookie、API Token |
| RBAC | 普通用户、协作者、软件管理员、站点管理员 |
| App | 应用提交、更新、下架、删除、基本信息修改 |
| Review | 应用上架、版本发布、应用信息变更审批 |
| Version | LPK 上传、外部 URL 版本、SHA256、版本保留清理 |
| Storage | local、WebDAV、S3、GitHub external-link |
| Visibility | 用户群组、私有应用可见性过滤 |
| Social | 评论、反馈、收藏、过期标记、协作者申请 |
| Taxonomy | 分类、标签、聚合分类 |
| Source | `/source/v1/index.json`、访问密码、密码轮转、镜像字段 |
| Admin | 用户、配置、分类标签、审核队列管理 |

### 数据模型

| 实体 | 说明 |
|---|---|
| User | 账号、角色、邮箱验证状态 |
| APIToken | CI/CD Bearer Token |
| App | 应用基本信息、审核状态、免审批开关、评论开关 |
| AppVersion | 版本号、下载地址、SHA256、发布状态、源类型 |
| AppScreenshot | 应用截图与排序 |
| Category / Tag | 分类和标签 |
| Collection / CollectionApp | 动态或手动聚合分类 |
| Collaborator / CollaboratorRequest | 协作者授权流程 |
| UserGroup / GroupMember / AppVisibility | 群组与私有应用可见性 |
| Comment | 评论与反馈 |
| Favorite | 应用和提交者收藏 |
| OutdatedMark | 用户过期标记 |
| ReviewRequest | 待审批事项 |
| SiteSetting | 站点运行时配置 |

## 客户端规划

### 技术栈

- Vite + React + TypeScript
- `@lazycatcloud/sdk`
- REST API 客户端与本地软件源订阅状态

### 导航结构

| Tab | 能力 |
|---|---|
| 首页 | 推荐 Banner、聚合分类、最近更新、下载最多 |
| 分类 | 分类筛选、标签和提交者维度 |
| 搜索 | 应用名、标签、提交者搜索 |
| 软件源 | 多源订阅、访问密码、GitHub 镜像 |
| 我的 | 登录注册、收藏、已安装、提交应用、群组、管理入口 |

### 安装链路

1. 客户端选择版本。
2. 若源版本包含 `upstreamDownloadUrl` 且客户端配置 GitHub 镜像，则生成镜像下载 URL。
3. 优先调用 LazyCat SDK `InstallLPK`，传入 `lpkUrl`、`sha256`、`pkgId`、`tmpTitle`。
4. 普通浏览器 fallback 必须先 fetch LPK 并计算 SHA256，校验失败或 CORS 阻止时中止。

## 部署规划

| 场景 | 命令 |
|---|---|
| 本地服务端 | `go run ./cmd/store-server` |
| 服务端裸应用 | `cd lazycat/server && lzc-cli project release -o ../../dist/lazycat-community-appstore-server.lpk` |
| 独立客户端裸应用 | `cd lazycat/client && lzc-cli project release -o ../../dist/lazycat-community-appstore-client.lpk` |
| 客户端预置默认源 | `CLIENT_DEFAULT_SOURCE_URL=... cd lazycat/client && lzc-cli project release -o ../../dist/lazycat-community-appstore-client.lpk` |

关键环境变量：

- `DB_DRIVER=sqlite3|postgres|mysql`
- `DB_DSN=...`
- `STORAGE_BACKEND=local|webdav|s3|github`
- `SITE_MAX_LPK_SIZE`
- `SITE_MAX_VERSIONS`
- `SOURCE_PASSWORD`
- `SOURCE_PASSWORD_ROTATION`
- `GITHUB_MIRROR`

## 开发里程碑

| 阶段 | 目标 | 验收 |
|---|---|---|
| M1 | 服务端基础、ent 模型、SQLite 启动 | `go test ./...` |
| M2 | 用户、权限、应用、版本、审核 | 管理员可审批用户提交版本 |
| M3 | 存储、SHA256、源端点、镜像 | 源 feed 能返回可安装版本 |
| M4 | 客户端浏览、订阅、详情、安装 | SDK 调用或校验后下载路径可走通 |
| M5 | 评论、收藏、协作者、群组、过期标记 | 权限边界测试通过 |
| M6 | LazyCat 裸应用、OpenAPI、CI | LazyCat YAML 和 OpenAPI 校验通过 |

## 验收清单

- `go test ./...`
- `cd client && npm audit --audit-level=high --registry=https://registry.npmjs.org`
- `cd client && npm run build`
- `npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml`
- `npx --yes js-yaml lazycat/server/package.yml`
- `npx --yes js-yaml lazycat/server/lzc-manifest.yml`
- `npx --yes js-yaml lazycat/server/lzc-deploy-params.yml`
- `npx --yes js-yaml lazycat/server/lzc-build.yml`
- `npx --yes js-yaml lazycat/client/package.yml`
- `npx --yes js-yaml lazycat/client/lzc-manifest.yml`
- `npx --yes js-yaml lazycat/client/lzc-build.yml`
- `npx --yes js-yaml .github/workflows/ci.yml`

## 已知边界

- 本地没有 `lzc-cli` 时，只能验证 LazyCat YAML 语法，不能实际构建或安装 LPK。
- LazyCat SDK 的真实安装动作需要在 LazyCat WebShell/微服环境内最终验收；普通浏览器只覆盖校验下载 fallback。
- `GET /api/v1/apps` 当前是面向中小型私有商店的列表接口，后续大规模公开站点应补游标分页。
