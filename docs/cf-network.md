# Cloudflare 优选连接

服务端 `0.1.50`、客户端 `0.1.45` 增加可选的优选 IP／域名连接。

## 使用

管理员在服务端设置的「Cloudflare 优选地址预设」中每行填写一个公网 IP 或域名。默认值为 `saas.sin.fan`；留空停止下发预设。域名可以通过 DNS 自动更新 IP。

客户端同步软件源后，在「设置 → 网络加速」选择内置默认、已同步的服务端预设或自定义地址，再启用并保存。客户端默认关闭加速，默认地址为 `saas.sin.fan`。服务端只提供候选配置，同步不会覆盖用户选择或自动启用。

优选连接适用于使用 Cloudflare 的 HTTPS 软件源目录、评论、心愿、聊天 API 和 SSE。通过客户端 Go 服务建立连接，原始 URL、HTTP Host、TLS SNI、证书校验和 HTTP/2 均保留。新连接会重新解析优选域名；已有连接继续复用。HTTP、不同源的重定向、LPK 下载及第三方图片不使用这项设置。优选地址本身通过公网 IP 校验，包括 DNS 解析结果。

优选 DNS、TCP 或 TLS 连接失败时，在发送 HTTP 请求之前尝试直接连接原站；不重放已发送的业务请求，也不因 HTTP 错误状态切换连接。启用后，优选链路及其回退直连绕过环境变量中的代理设置，以便实际控制连接地址。关闭时恢复原有 HTTP 客户端行为。

## 配置接口

- `PATCH /api/v1/admin/settings`：字符串设置 `cf_preferred_endpoints`，最多 32 个去重后的地址，支持换行或逗号分隔。
- `/source/v2/index.json`：`site.clientPolicy.cfPreferredEndpoints` 为可选字符串数组；未提供或空数组表示没有额外预设。
- `GET /api/client/v1/settings`：`settings.cfEnabled`、`settings.cfEndpoint` 和只读 `settings.cfPresets`（`endpoint`、`sourceName`）。内置默认项的 `sourceName` 为空。
- `PATCH /api/client/v1/settings`：在原有完整设置请求中增加 `cfEnabled`、`cfEndpoint`。省略这两个字段保留原有网络选项。非法地址返回 `400 INVALID_CF_ENDPOINT`，且在写入任何设置之前拒绝请求。

客户端选择及接收到的预设保存在已有设置表中，按用户隔离，无需数据库结构迁移。旧服务端缺少预设字段时，客户端仍可选择内置默认或自定义地址。

## 可重复测试

```bash
go test ./internal/cfnetwork ./internal/clientserver ./internal/server
node --test client/src/modules/client/cfNetworkState.test.mjs
CF_NETWORK_TEST=1 CF_REQUIRE_FASTER=1 go test -v ./internal/cfnetwork -run TestLive -count=1
CF_NETWORK_TEST=1 CF_REQUIRE_FASTER=1 CF_TEST_ENDPOINT=172.64.229.66 go test -v ./internal/cfnetwork -run TestLive -count=1
```

真实网络测试固定请求 `https://apps.pushcat.eu.org/source/v2/index.json`，交替执行三轮直连和优选连接，每次使用新连接、完整读取并验证 JSON，同时记录远端 IP、应用数、字节数及 SHA-256。优选测试禁止直连回退。`CF_REQUIRE_FASTER=1` 会在优选中位耗时不低于直连时失败；默认离线测试不依赖外网速度。

2026-09-08，本工作环境最终实现的实测结果：

| 优选地址 | 直连中位耗时 | 优选中位耗时 | 降低 |
| --- | ---: | ---: | ---: |
| `saas.sin.fan` | 2.923 秒 | 2.270 秒 | 22.3% |
| `172.64.229.66` | 2.924 秒 | 2.152 秒 | 26.4% |

上述 12 次响应均为 797,648 字节、260 个应用，SHA-256 均为 `0666227e88becfb166a1008a01d2a73fcf3797d2ba3b28b81b401d619160c2d8`。直连使用原站 DNS 返回的 IPv6，域名优选当时连接 `172.64.229.235`。结果反映该网络与测试时段，其他网络应重新测试；IP 不是固定推荐值。
