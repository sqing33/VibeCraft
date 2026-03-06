---
name: expert-creator
description: 将用户对“专家”的自然语言需求转成结构化专家配置草稿，适用于 vibe-tree 的专家设置页与 AI 生成流程。
---

# Expert Creator

## 目标

- 从自然语言需求生成可发布的专家配置草稿。
- 约束专家边界，避免生成“万能助手”。
- 使用最小必要 skill 集合与清晰的主副模型策略。

## 输入

- 用户对专家的需求描述
- 可用模型 ID 列表
- 可用 skill ID 列表
- （可选）上一版专家草稿

## 输出

至少输出以下字段：

- `id`
- `label`
- `description`
- `category`
- `primary_model_id`
- `secondary_model_id`
- `system_prompt`
- `enabled_skills`
- `fallback_on`
- `output_format`

## 生成规则

1. 专家必须聚焦单一领域，禁止输出泛化型全能助手。
2. 主模型/副模型必须来自给定模型列表。
3. 优先选择最小 skill 集合，不要默认全部启用。
4. `system_prompt` 必须说明角色、优先级、边界与输出偏好。
5. 若需求不完整，应在说明里写清假设，而不是留空关键字段。
6. 若不需要副模型，允许 `secondary_model_id` 留空。

## 推荐流程

1. 识别专家类别（design / coding / planning / ops / research / general）
2. 总结最核心的 3 个目标
3. 选择主模型与是否需要副模型
4. 选择 skill
5. 生成 system prompt
6. 生成输出格式与 fallback 策略

## 自检

- ID 为 kebab-case
- 模型引用合法
- prompt 没有跨域失焦
- skill 足够少、足够准
- 输出可直接进入专家设置页发布
