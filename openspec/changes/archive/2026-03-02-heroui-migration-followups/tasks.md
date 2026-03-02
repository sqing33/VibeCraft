## 1. LLM 设置页校验收尾

- [x] 1.1 在 `ui/src/app/components/LLMSettingsTab.tsx` 中禁止清空 Model 的 Source 选择（`disallowEmptySelection` + 选择回退）
- [x] 1.2 在无 Source 时阻止新增 Model，并在保存前阻止提交 `source_id` 为空的配置（toast 提示）

## 2. Tailwind 配置清理与验证

- [x] 2.1 清理 `ui/tailwind.config.cjs` 中未使用的 `accordion-*` keyframes/animation
- [x] 2.2 在 `ui/` 目录执行 `npm run lint` 与 `npm run build`
