# Capability Map: 管理员应用维护增强

| Module id | Responsibility | Depends on |
|---|---|---|
| `github-update-management` | 管理员单应用与批量启用 GitHub LPK 自动更新，支持社区点分数字版本 | 现有 GitHub LPK 策略 API |
| `collection-app-search` | 创建和编辑合集时按应用名、包名和 slug 搜索可选应用 | 现有合集应用选择器 |

Build order: `github-update-management` → `collection-app-search` → 前端嵌入产物与服务端发布。
