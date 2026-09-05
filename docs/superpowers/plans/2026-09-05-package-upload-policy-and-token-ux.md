# 软件包上传策略与 API Token 交互

## 已确认范围

- `package-upload-policy`：站点管理员可以关闭软件包文件上传，默认允许。关闭后网页创建应用和发布版本只显示 URL 输入，API 在读取 multipart 软件包之前返回 403；JSON 创建应用必须带下载 URL。截图、头像、存量下载不受影响。
- `api-token-ux`：保持现有界面样式，允许命名 Token；区分加载、失败、空列表；生成后单独显示一次性密钥并支持复制和手动保存；防止重复提交，撤销时确认目标及影响。帮助统一使用 URL 发布。
- `server-release`：上述功能验证通过后更新服务端补丁版本，提交、推送并发布服务端 tag，核实远端发布结果。客户端版本独立维护，本次变更位于服务端商店界面。

## 实现与验收顺序

1. 复用 `internal/server/settings.go` 的 SiteSetting，增加 `allow_package_upload` 布尔配置和公开 `site.packageUpload.allowed` 能力；更新管理接口校验、应用入口和 OpenAPI。按仓库 Go 风格实现，不新增依赖或数据库表。
2. `client/src/modules/admin` 增加开关；通过 `SiteProfile` 将能力传给个人提交表单和版本发布对话框，关闭时从本地模式切换为 URL 并清理已选软件包。
3. 改善 `APITokenWorkspace` 的命名、保存、复制、撤销、加载重试和移动端布局，补齐中英文文案。
4. 测试默认、关闭、重新开启、普通用户和 Token 调用、URL 可用及截图不受影响；浏览器检查 Token 创建、复制、撤销、加载失败及窄屏。
5. 执行 `go test -race ./...`、`go vet ./...`、`node --test $(rg --files client/src -g '*.test.mjs')` 和 `npm --prefix client run build`，审查当前完整 diff 后沿用 `.github/workflows/lazycat-release.yml` 发布。

## 边界

沿用 React/Astryx 组件与现有主题，不增加依赖或改动认证权限模型。Token 明文只保存在页面内存，不写入持久存储或日志。用临时数据目录进行浏览器验证，保留现有工作目录数据。
