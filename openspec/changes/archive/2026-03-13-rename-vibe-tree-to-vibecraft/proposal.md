## Why

当前仓库仍大量使用早期临时名称 `vibe-tree`，这已经和产品对外定位不一致，也会持续影响文档、路径、可执行名和用户认知。现在需要一次性完成从旧名到 `VibeCraft` 的收口，避免后续功能继续建立在过时命名之上。

## What Changes

- 将项目的对外产品名称统一更新为 `VibeCraft`
- 将仓库内所有用户可见文案、文档标题、桌面应用名称和说明文字从旧名切换到新名
- **BREAKING** 将运行相关的目录、可执行名、配置路径、环境变量前缀和默认数据路径从 `vibe-tree` 迁移到 `vibecraft`
- **BREAKING** 重命名当前工作目录与仓库引用路径，确保脚本、文档和构建入口都指向新目录
- 为旧路径迁移提供兼容或迁移处理，避免已有本地数据与配置直接失效

## Capabilities

### New Capabilities
- `product-rename`: 统一定义产品名、运行标识、默认路径和迁移行为，覆盖从 `vibe-tree` 到 `VibeCraft` 的全链路重命名

### Modified Capabilities
- `ui`: UI 中的产品名称、说明文字和入口展示需要切换到新品牌名
- `workflow`: 默认运行目录、日志路径和相关说明需要反映新的产品标识
- `repo-library-ui`: Repo Library 相关展示中的宿主产品名称需要切换到新品牌名
- `cli-runtime`: CLI runtime 的默认环境变量、运行时提示和宿主产品标识需要切换到新名前缀

## Impact

- 影响 `backend/`、`ui/`、`desktop/`、`services/`、`scripts/`、`docs/`、`openspec/`
- 影响默认配置目录、数据目录、日志目录、可执行文件名、环境变量前缀和仓库目录名
- 需要验证 Go 构建、前端构建、桌面壳配置、Python repo analyzer 和文档引用是否在新名称下保持可用
