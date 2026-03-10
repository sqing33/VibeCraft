# Design: auto-detect-thinking-translation

## Overview

本次改动把“思考过程翻译”从“按目标模型列表启用”重构为“按思考内容自动启用”。整体原则是：
- 配置层只保存翻译模型。
- 运行时按 thinking 内容判断是否需要翻译。
- 已经是中文主导的内容不再调用翻译模型。
- UI 只暴露用户真正需要理解和控制的配置项。

## Configuration model

`basic.thinking_translation` 只保留 `model_id` 作为有效配置字段。

兼容策略：
- 继续兼容历史配置中的 `source_id` / `model` / `target_model_ids` 反序列化。
- 规范化与校验阶段忽略 `target_model_ids`，并在重新保存时不再写出。
- 如果翻译模型不存在或不是可用的 SDK helper 模型，则清空整项配置。

## Runtime decision flow

在 `thinkingTranslationRuntime` 中为每个 thinking entry 维护一次自动判断结果：
- `unknown`：尚未决定是否翻译。
- `translate`：该 entry 后续片段都进入翻译流程。
- `skip`：该 entry 后续片段都不翻译。

判断时机：
- 当 entry 出现首个可发布 chunk（句子边界 / 长度阈值 / turn complete）时，基于该 chunk 做一次本地语言判断。
- 如果判断为“中文主导”，则整条 entry 跳过翻译。
- 如果判断为“非中文主导”，则继续沿用现有 chunked translation 流程。

## Language heuristic

采用纯本地启发式规则，避免额外网络延迟：
- 统计 Han、Latin、Kana、Hangul 以及其他字母脚本字符数。
- 含有明显 Kana/Hangul/其他非中文脚本时，倾向认定为需要翻译。
- Han 占比足够高且总体呈中文主导时，认定为无需翻译。
- 纯符号/纯空白内容不触发翻译。

该策略的目标不是精确识别“英文”，而是识别“是否已经是中文主导，不需要再翻译成中文”。

## Applied flag semantics

`thinking_translation_applied` 需要改成“实际产生过翻译输出”。

因此：
- 仅配置了翻译模型，但整轮都被自动判定为中文主导时，`thinking_translation_applied=false`。
- 只要任一 thinking entry 实际产生了翻译 delta，就视为 `thinking_translation_applied=true`。
- `thinking_translation_failed` 仍表示翻译调用过程中实际失败。

## UI changes

基本设置页保留：
- `翻译模型`

移除：
- `需要翻译的 AI 模型`

文案改为说明系统会自动判断思考过程是否需要翻译为中文。
