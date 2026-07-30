# 许愿墙、目录排序与下游封禁总计划

已确认规格：[docs/spec-wish-wall-and-client-access.md](../docs/spec-wish-wall-and-client-access.md)

实现按依赖顺序拆成三个可独立验证的功能计划，以及一个发布收尾计划：

1. [目录更新时间与下载量排序](../docs/superpowers/plans/2026-07-30-client-catalog-download-sorting.md)
2. [许愿墙服务端、客户端代理与界面](../docs/superpowers/plans/2026-07-30-wish-wall.md)
3. [下游客户端用户封禁](../docs/superpowers/plans/2026-07-30-downstream-client-blocking.md)
4. [双端版本、tag、推送与发布验证](../docs/superpowers/plans/2026-07-30-dual-release.md)

依赖关系：计划 1 提供 feed/cache 扩展模式；计划 2 提供客户端用户身份和许愿墙记录；计划 3 复用该身份完成源入口封禁；计划 4 只在前三项和全量验证通过后执行。
