# Cloudflare 优选地址

## 目标与验收
- 服务端管理员配置 IP／动态优选域名列表，通过软件源 site.clientPolicy.cfPreferredEndpoints 下发；内置 saas.sin.fan。
- 客户端用户在设置中启用／关闭、选择预设或自定义地址；默认地址 saas.sin.fan，默认不启用，保留现有连接行为。
- 优选连接只作用于软件源 HTTPS 同源请求（目录、评论、心愿、聊天 API 与 SSE）；保持 URL、Host、SNI 和证书验证。HTTP、跨站重定向继续使用正常链路。
- DNS 每次新建连接解析，拒绝非公网目标。连接／TLS 失败时在发送 HTTP 请求前回退直连，不重放业务请求。
- 预设和客户端选择持久化，按用户隔离。旧服务端无新字段仍可使用内置默认值。
- 自动化验证路由、TLS、回退、隔离、配置下发和保存；可选真实网络测试使用 https://apps.pushcat.eu.org/source/v2/index.json，严格关闭回退并对比三轮完整 JSON 响应和中位耗时。不能把“可用”报告成“更快”。

## 实现顺序
1. internal/cfnetwork：共享输入校验与优选 HTTP transport。
2. internal/server、internal/feed：配置保存与下发；internal/clientserver：接收、持久化、请求路由。
3. client/src：现有 React 设置组件、中英文文案、脏状态与保存行为。
4. 单元／集成测试、实际网络对比、构建与检查，更新 lazycat/{server,client}/package.yml，提交并创建独立版本 tag。

## 命令
- go test ./...
- go test -race ./...
- go vet ./...
- cd client && npm run build
- node --test client/src/modules/client/*.test.mjs
- CF_NETWORK_TEST=1 go test -v ./internal/cfnetwork -run TestLive

## 约束与风格
使用 Go 1.26、标准库 HTTP、现有 Ent key/value settings 和 React/Astryx 组件；不增加依赖或数据库 schema。Go 使用 gofmt、context 和有界超时，例如 `req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)`。所有输入在写入之前验证；不关闭证书检查，不暴露请求凭据。沿用现有 UI 风格和字段标签，错误贴近字段，异步操作显示状态。
