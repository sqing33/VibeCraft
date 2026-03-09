## Why

当前系统已经能以 JSON 方式保存 MCP，并能从项目/用户目录发现 Skill，但设置页仍存在三个明显问题：

- MCP 页面信息密度过低，说明文案占位过大，卡片与默认开关布局浪费空间。
- Skill 页面只有“发现列表”，缺少真正可用的总开关与安装入口，无法把手工添加的 Skill 纳入后续 Codex 使用流程。
- MCP / Skill 页顶部工具条会跟着内容一起滚动，长列表下操作效率差。

用户希望把这两块能力改成真正可用的“Codex 注入准备区”：MCP 继续走 JSON 直配；Skill 支持发现、启停与安装；两页都采用固定头部 + 内容区滚动的紧凑布局。

## What Changes

- 重构 MCP 设置页：
  - 移除顶部大段说明文案。
  - 顶部统计/新增/保存工具条固定。
  - MCP 卡片改为双列布局。
  - 每张卡片内的 CLI 默认开关改为紧凑的两列小网格。
  - 新增 MCP 时默认内容为空，使用 Context7 MCP JSON 作为灰色 placeholder 示例。
- 重构 Skill 设置页：
  - 恢复每个 Skill 的单一启用/关闭开关。
  - 移除按 CLI 工具的 Skill 开关模型。
  - 增加“添加 Skill”能力，支持上传 zip 或选择文件夹安装。
  - 安装后的 Skill 放入用户级 `~/.codex/skills/<skill-id>/`，供后续 Codex 运行时发现与注入。
- 扩展后端 Skill 设置能力：
  - `GET /api/v1/settings/skills` 返回 discovered skills 与 enabled 状态。
  - `PUT /api/v1/settings/skills` 持久化 Skill 总开关。
  - `POST /api/v1/settings/skills/install` 支持 zip / 文件夹安装。
- 调整 Codex 运行时 Skill 注入：
  - 默认仅注入“已发现且已启用”的 Skill。
  - 若 expert 声明 `enabled_skills`，则在已启用集合上进一步收窄。

## Capabilities

### Modified Capabilities

- `mcp-skill-settings`: MCP / Skill 设置页改为更紧凑的可操作布局，并支持 Skill 启停与安装。
- `cli-runtime`: Codex 运行时 Skill 注入从“全部发现”收敛为“发现 ∩ 启用 ∩ expert.enabled_skills（可选）”。
- `ui`: 系统设置弹窗中的 MCP / Skill Tab 采用固定头部 + 内容区滚动。

## Impact

- 后端：新增 Skill 安装处理与配置持久化逻辑；更新 Skill 响应与运行时过滤。
- 前端：调整 `MCPSettingsTab` / `SkillSettingsTab` / `SettingsDialog` 结构与交互；补充上传 API。
- 配置：`skill_bindings` 语义收敛为“按 skill id 维护 enabled 状态与显示元数据”，不再包含 per-tool 绑定。
