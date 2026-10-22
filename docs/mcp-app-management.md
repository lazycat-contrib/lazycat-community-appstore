# 管理 MCP：软件与版本维护

使用服务端用户设置页创建的 **ADMIN 类型 MCP token**。绑定账号还必须保持管理员角色且未停用；绑定管理员账号的普通 USER token 不会获得这些能力。

以下工具均通过 `appId` 或 `packageId` 指定软件，二者只能提供一个。

| 工具 | 用途 |
| --- | --- |
| `appstore_admin_get_app` | 查看软件信息与版本保留策略，包括尚未公开的软件 |
| `appstore_admin_update_app` | 修改名称、摘要、描述、评论/邮件通知开关、免审核更新开关或安装密码 |
| `appstore_admin_unlist_app` | 下架软件，保留软件、版本和关联数据 |
| `appstore_admin_delete_app` | 永久删除软件、版本、截图及关联记录，需要 `confirm: true` |
| `appstore_admin_list_versions` | 分页列出所有版本，包含未审核或已拒绝版本；按 `nextPage` 继续读取 |
| `appstore_admin_update_version` | 用 `versionId` 修改对应版本的 `changelog` |
| `appstore_admin_delete_version` | 删除指定软件的版本，需要 `versionId` 与 `confirm: true`，返回可能的存储清理警告 |
| `appstore_admin_set_version_retention` | 设置版本保留策略，需要 `confirm: true`；可能立即删除较旧的已审核版本 |

部分更新会保留未提供的字段；显式传入空摘要/描述可清空，`false` 会关闭对应开关。软件信息编辑不会自动改变审核状态；安装密码不会出现在返回数据中。

分类、标签继续使用已有的 `appstore_admin_set_app_category`、`appstore_admin_set_app_tags` 等工具。新增软件和发布新版本继续使用 `appstore_create_app_from_url`、`appstore_publish_version_from_url`。

版本号、安装包内容、下载地址和校验值不通过“编辑更新说明”工具改写。包内容变化应发布新版本，避免客户端缓存和校验结果与已有版本不一致。

## 示例参数

修改软件信息：

```json
{"packageId":"community.example.app","summary":"新的简介","commentsEnabled":false}
```

修改版本更新说明：

```json
{"appId":123,"versionId":456,"changelog":"修复连接问题"}
```

保留最新三个已审核版本（会立即清理较旧版本）：

```json
{"appId":123,"mode":"CUSTOM","maxVersions":3,"confirm":true}
```

`CUSTOM` 的 `maxVersions: 0` 表示全部保留；`INHERIT` 使用站点策略，必须省略 `maxVersions`。删除操作不可撤销，仅下架时优先使用 `appstore_admin_unlist_app`。
