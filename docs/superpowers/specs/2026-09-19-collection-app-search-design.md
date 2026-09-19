# Spec: 合集应用搜索

## Objective

管理员创建或编辑软件合集时，可在现有应用勾选器中即时搜索，避免在长列表中手工翻找。

## Tech Stack

- React 19、TypeScript 5.9、AstryX Design 输入与复选组件。
- 不修改合集 API、Ent schema 或已选 `appIds` 的保存语义。

## Commands

```bash
node --test --experimental-strip-types client/src/modules/admin/collectionAppPickerState.test.mjs
(cd client && npm run build)
```

## Project Structure

- `client/src/modules/admin/CollectionAppPicker.tsx`: 搜索输入和过滤后的应用列表。
- `client/src/modules/admin/collectionAppPickerState.ts`: 可单测的搜索匹配逻辑。
- `client/src/locales/`: 中英文占位和空结果文案。

## Code Style

搜索逻辑保持纯函数，不改变输入数组；界面继续使用现有 `XTextInput` 和 `XCheckboxInput`。

## Testing Strategy

- 单元测试覆盖默认名称、本地化名称、`packageId`、slug、大小写和空查询。
- 生产构建验证 TSX 类型和组件属性。

## Boundaries

- Always: 搜索只影响当前可见选项，不清除已选应用。
- Ask first: 服务端分页搜索、分类筛选、拖拽排序。
- Never: 修改合集保存请求的 `appIds` 顺序或自动取消隐藏的已选项。

## Success Criteria

- 创建与编辑合集共用同一搜索能力。
- 搜索可命中应用中英文名、包名和 slug。
- 无匹配时显示明确空状态，已选数量不变。

## Open Questions

无。
