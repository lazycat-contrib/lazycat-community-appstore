# 实施任务

- [ ] 计划 1：v2 feed 下发累计下载量，客户端缓存并默认按最近更新时间排序，可切换下载最多。
  - Acceptance：新旧 v2 源均可同步；无下载量按 0；默认 `recent`；排序稳定。
  - Verify：`go test ./internal/feed/... ./internal/server/... ./internal/clientserver/...`；客户端目录测试与构建。
- [ ] 计划 2：实现只允许已识别懒猫客户端提交的许愿墙、公开/私有权限、回复和带说明的状态历史。
  - Acceptance：服务端浏览器不能提交；匿名只能看两类公开条目；建议本人可见；状态说明不可为空且历史不可覆盖。
  - Verify：许愿墙服务端权限矩阵、客户端代理测试、前端契约测试与 OpenAPI 校验。
- [ ] 计划 3：管理员从评论/许愿墙定位并封禁下游客户端用户。
  - Acceptance：仅站点管理员可封禁；新客户端同步和互动返回 `CLIENT_BLOCKED`；旧客户端无身份头仍兼容。
  - Verify：封禁 API、源 feed、评论、过期标记、私信和客户端错误呈现测试。
- [ ] 计划 4：全量审查、版本更新、构建产物同步、提交、推送与双 tag 发布。
  - Acceptance：服务端 `0.1.39`、客户端 `0.1.32`；分支与 `server-v0.1.39`、`client-v0.1.32` 推送；两个发布工作流成功且 release 资产存在。
  - Verify：完整 Release Gate 2.0、CI 和 GitHub Release 回读。
