# Spec: GitHub LPK 自动更新检查

## Objective

为服务端应用维护者提供逐应用的 GitHub LPK 自动更新策略。仅当应用当前最新版本使用标准 GitHub Release LPK 直链时显示和允许启用；后台按配置间隔调用 GitHub Release API，发现新的 LPK Release Asset 后，使用 GitHub 返回的附件 SHA256 摘要自动创建或更新应用版本记录。

成功标准：

- `https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}.lpk` 可启用自动检查，其他地址不显示该选项且后端拒绝启用。
- 策略存储在独立表中，与应用一对一关联，并由数据库外键在删除应用时级联删除。
- 关闭自动检查时，界面的间隔输入保持可见但禁用，后台不再调度。
- 默认间隔为 24 小时，允许 1 小时到 30 天。
- 使用 `github.com/google/go-github/v89/github` 的 `Repositories.GetLatestRelease` 和 `ReleaseAsset.Digest`。
- 不下载 LPK，不使用 GitHub 镜像，不自行计算 SHA256。
- 附件缺少合法 `sha256:<64 hex>`、附件匹配不唯一、API 失败或地址失效时，不改变现有版本，只记录错误并在下个周期重试。
- 自动版本继续遵守应用现有审核策略：管理员或允许免审更新的应用直接发布，否则创建待审核版本。
- 自动写入版本号、Release 说明、附件直链、SHA256、文件大小和独立的 GitHub 上游发布时间；商店发布时间仍用于最新版本排序和版本保留。
- 完成后发布服务端 `server-v0.1.38` tag，并验证 GitHub Release LPK 附件可用。

## Tech Stack

- Go 1.26.4、Ent v0.14.6、SQLite/PostgreSQL/MySQL。
- `github.com/google/go-github/v89/github` 访问 GitHub Releases API。
- React 19、TypeScript 5.9、Vite 7、AstryX Design 组件。
- GitHub Actions 的 `server-v*` 发布流水线构建并发布服务端 LPK。

## Commands

```bash
# Ent 代码生成
go run entgo.io/ent/cmd/ent@v0.14.6 generate ./ent/schema

# Go 格式、测试和静态检查
gofmt -w <changed-go-files>
go test ./...
go vet ./...
go test -race ./...
go mod tidy -diff

# 前端构建和嵌入资源同步
(cd client && npm ci)
(cd client && npm run build)
diff -qr --exclude=app-config.js client/dist clientembed/dist

# API 与 LazyCat 配置验证
npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml
npx --yes js-yaml lazycat/server/package.yml
npx --yes js-yaml lazycat/server/lzc-manifest.yml
npx --yes js-yaml lazycat/server/lzc-deploy-params.yml
npx --yes js-yaml lazycat/server/lzc-build.yml

# 发布
git tag -a server-v0.1.38 -m "MiaoMiao Server v0.1.38"
git push origin main
git push origin server-v0.1.38
```

## Project Structure

```text
ent/schema/                         Ent 数据模型；新增一对一自动更新策略表
internal/server/                    HTTP API、GitHub 客户端、调度器和版本生命周期
client/src/modules/storefront/     应用管理界面和策略设置交互
client/src/shared/                 前端 API 类型与 GitHub URL 判定工具
client/src/locales/                 中英文界面文案
docs/openapi.yaml                   服务端 API 合同
clientembed/dist/                   服务端编译时嵌入的前端构建产物
lazycat/server/                     服务端 LazyCat 包版本与发布配置
```

## Code Style

Go 代码使用小型解析/校验函数和可注入客户端，错误保留上下文但不暴露内部凭据：

```go
func parseGitHubReleaseLPKURL(rawURL string) (githubReleaseLPK, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return githubReleaseLPK{}, errors.New("a GitHub Release LPK URL is required")
	}
	// Validate the exact /owner/repo/releases/download/tag/asset.lpk shape.
	return release, nil
}
```

- Go：导出类型只在跨文件/包需要时使用；后台任务接受 `context.Context`；时间使用服务器可注入时钟。
- TypeScript：沿用现有 `StoreApp`/管理抽屉状态模式；组件属性和 API DTO 显式声明类型。
- JSON 字段使用 camelCase；数据库字段使用 snake_case。

## Testing Strategy

- 单元测试 GitHub Release URL 解析、Release tag 版本提取、SHA256 digest 解析和多附件匹配。
- 使用可注入的 `go-github` Release 客户端验证最新 Release 请求、成功更新、缺少 digest、重复附件、API 错误和无降级行为。
- 调度器测试启用后立即到期、禁用不运行、成功/失败更新时间、关闭服务器可取消后台任务。
- API 测试权限、间隔边界、非 GitHub 地址拒绝和 DTO 状态。
- 数据库测试直接删除应用后策略记录由外键级联删除。
- 前端以 TypeScript 构建覆盖字段、条件显示和禁用态；最终运行完整 Go/React/配置验证。

## Boundaries

- Always：只处理公共 GitHub Release 直链；验证 owner/repo/tag/asset；要求附件名以 `.lpk` 结尾；要求 GitHub digest 为 SHA256；保留原有审核、版本保留和软件源失效机制。
- Ask first：支持私有仓库 Token、预发布 Release、自定义附件匹配规则、非 GitHub 上游或下载 LPK 解析包内元数据。
- Never：通过镜像或直链下载 LPK 计算摘要；在 digest 缺失时发布未校验版本；绕过版本审核；把 GitHub API 错误暴露为敏感内部信息。

## Detailed Behavior

### Policy model

新增 `github_lpk_update_policies` 表：

- `app_id`：唯一外键，引用 `apps.id`，`ON DELETE CASCADE`。
- `enabled`：是否参与调度。
- `interval_minutes`：默认 1440，允许 60 到 43200。
- `last_checked_at`、`last_success_at`、`next_check_at`：调度状态。
- `last_version`：最近一次成功同步或确认的 GitHub Release 版本。
- `last_error`：最近一次失败说明；成功后清空。
- `created_at`、`updated_at`。

禁用策略不删除记录，以保留上次间隔和检查状态；删除应用依赖数据库级联删除策略记录。

调度使用带 `enabled` 和到期条件的原子 claim；取消任务时释放 claim，完成写回使用条件更新，避免多实例重复执行或覆盖用户并发修改。重新启用时仍遵守最近检查时间和配置间隔，不能通过反复开关绕过最短周期。

### Supported URL

仅支持：

```text
https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}.lpk
```

不支持仓库首页、Release 页面、`latest/download`、GitHub API URL、raw URL、查询参数改变资源身份的 URL 或非 `.lpk` 文件。

### Release and asset selection

1. 从当前最新版本下载地址解析 owner、repo、当前 tag 和当前 asset 文件名。
2. 调用 `Repositories.GetLatestRelease`，它只返回最新正式 Release，不包含 draft/prerelease。
3. 在 Release Assets 中筛选 `.lpk`：
   - 优先匹配“将当前文件名中的当前 tag/当前版本替换为最新 tag/版本”后的文件名；
   - 其次匹配唯一包含应用 `packageId` 的 `.lpk`；
   - 最后仅当 Release 中只有一个 `.lpk` 时使用它；
   - 仍有多个候选时记录歧义错误，不更新。
4. 只接受 `digest` 为 `sha256:<64 hex>` 的附件。

### Version update

- 版本号从 Release tag 中提取 SemVer 主体（例如 `v2.0.3`、`release-2.0.3`、`server-v0.1.38`），通过 SemVer canonicalization 规范化后，以不带前导 `v` 的形式入库；无法得到合法版本时不更新。
- 若目标版本低于当前已发布版本，视为无更新，不降级。
- 若目标版本相同且 URL、digest、大小和 changelog 均相同，只更新策略成功状态。
- 若目标版本相同但附件发生变化：仅管理员/免审应用可更新现有记录；其他应用记录错误并要求人工处理，避免把当前已发布版本降为待审核。
- GitHub 发布时间写入 `upstream_published_at`；`published_at` 表示版本在商店中的实际发布时间，避免较早的上游时间导致新 SemVer 被排序为旧版本或被版本保留误删。
- 已待审核的版本内容不可被后台替换，后续开启免审也不会追溯批准；已拒绝版本不会自动重新提交。
- 同版本只允许更新同一 GitHub 仓库、同一附件名的记录；跨来源、跨仓库或不同附件必须人工处理。
- 新版本复用现有版本发布生命周期：免审则批准、清除过期标记并执行版本保留；否则创建待审核版本和审核请求。
- 仅 `APPROVED` 应用执行自动检查，任务发布前会重新验证策略、应用、所有者和当前最新已批准版本，避免下架、禁用、关闭免审或切换上游后的陈旧任务落库。
- 自动检查不会修改应用名称、描述、图标、作者、主页、许可证或最低系统版本。

### API and UI

- 应用详情仅向有管理权限的用户返回 `githubLPKUpdatePolicy`，且当前最新版本 URL 支持时才返回。
- 新增策略更新接口，只有应用所有者或管理员可调用；后端再次验证 URL 和间隔。
- 首次启用时安排检查并唤醒调度器；重新启用时遵守最近检查时间和配置间隔，禁用时清空 `nextCheckAt`。
- 管理页显示开关、以小时为单位的间隔输入、最近检查/成功时间、下次检查和最近错误。
- 非 GitHub Release LPK 地址完全不渲染该操作卡；关闭开关时区间输入禁用。

## Success Criteria

- 所有测试和构建命令通过。
- 数据库级联删除测试通过。
- `go.mod` 使用 `github.com/google/go-github/v89`，代码直接读取 `ReleaseAsset.GetDigest()`。
- 代码中不存在为该自动更新功能调用 `inspectLPKURL`、GitHub mirror rewrite 或下载附件计算摘要的路径。
- `server-v0.1.38` tag 指向包含本功能和服务端版本号 `0.1.38` 的提交。
- GitHub Actions 发布成功，Release 附件名为 `community.lazycat.app-store-server-v0.1.38.lpk`，且远端附件带有效 SHA256 digest。

## Open Questions

无。私有仓库 Token、预发布和非 GitHub 上游明确不在本次范围。
