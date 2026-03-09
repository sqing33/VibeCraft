## Context

当前 MCP 已经采用 JSON 原样编辑并落盘到 `config.mcp_servers`，运行时通过 `config.EffectiveMCPServers` 注入到 Codex 的 `mcp_servers` 配置里。Skill 则通过 `skillcatalog.Discover()` 从项目与用户目录扫描 `SKILL.md`，并在 `codex_runtime_settings.go` 里把发现结果拼成 base instructions 的 allowlist。

问题在于：

1. Skill 只有“发现”没有“管理”，用户无法控制是否参与运行时注入。
2. Skill 也没有“安装”能力，用户必须手工去目录里拷贝内容。
3. 当前 MCP / Skill 页整体排版偏松散，信息密度不符合用户预期。

## Goals / Non-Goals

**Goals**

- Skill 设置支持全局单开关（按 skill id）。
- Skill 支持 zip / 文件夹安装到用户级 `~/.codex/skills`。
- 运行时只注入已启用 Skill，并可被 expert 的 `enabled_skills` 继续收窄。
- MCP / Skill 两页都做到顶部工具条固定，下面内容单独滚动。
- MCP 卡片与默认 CLI 开关显著压缩布局占用。

**Non-Goals**

- 不实现远程 Skill 市场或 GitHub 在线安装。
- 不实现 Skill 的 per-tool 开关。
- 不修改 Chat 页面当前 MCP 会话选择机制。
- 不直接改写用户全局 Codex 的其它配置文件。

## Decisions

1. **Skill 安装目标目录选用 `~/.codex/skills/<skill-id>/`**
   - 这是 Codex 默认技能目录，后续 CLI / app-server 可以自然发现。
   - 与项目级 `.codex/skills/` 并存，不影响项目内已有 Skill。

2. **Skill 启停仅持久化到 vibe-tree 自身配置 `skill_bindings`**
   - 不使用 Codex `skills/config/write` 去修改用户全局 Skill 配置，避免越界改动用户环境。
   - 运行时由 vibe-tree 根据 `skill_bindings` 过滤 discovered skills，再把有效技能清单注入 Codex 指令层。

3. **安装接口统一为 multipart 上传**
   - zip 上传：前端上传单个压缩包，后端解压到临时目录并定位 `SKILL.md`。
   - 文件夹上传：前端用 `webkitdirectory` 选目录后，把文件相对路径随 multipart 文件名一起提交，后端重建目录结构。

4. **安装时以 `SKILL.md` 所在目录为 skill 根目录**
   - 后端从临时目录里找出唯一 `SKILL.md`。
   - 解析 `name:` 作为最终安装目录名；若没有 `name:`，则退回根目录名。
   - 目标目录已存在时整体替换，保证“添加同名 skill”可视为更新。

5. **MCP 示例 placeholder 使用 Context7 的远程 MCP 形态，但字段采用 Codex 兼容写法**
   - 使用 `url` + `bearer_token_env_var`，避免直接展示仅对其它 MCP 客户端生效的 `headers` 结构。

## Risks / Trade-offs

- [Risk] Skill 安装 zip 可能包含路径穿越条目。
  - Mitigation：解压时严格校验目标路径必须落在临时目录内。
- [Risk] 用户关闭某个默认目录内的 Skill 后，Codex 底层仍可能从默认目录看到它。
  - Mitigation：当前产品层仅向 Codex 注入“有效技能列表”，把 enable/disable 明确作用于 vibe-tree 的运行时 allowlist；不直接篡改用户全局 Codex 配置。
- [Risk] 替换式安装可能覆盖用户旧版本 Skill。
  - Mitigation：只覆盖同名目标目录，且来源是用户主动操作“添加 Skill”。
