---
title: Frontmatter 演示索引
description: frontmatter 元数据的读取规则与人工验证入口，覆盖日期别名、标题来源与描述字段
---

# Frontmatter 演示索引

本目录用于人工验证创建/更新时间元数据的 frontmatter 读取变更：`create_date > create_at > create_time` 与 `update_date > update_at > update_time` 两组别名按固定优先级归一，工具栏摘要优先显示创建时间（CalendarPlus）与更新时间（CalendarSync），通用 `date` 仅在两者都不存在时兜底显示（CalendarDays）。

启动预览后逐个打开文件，对照工具栏摘要与 Frontmatter 表格：

```bash
m2h web docs/demos
```

| 文件 | 验证点 | 工具栏摘要预期 |
| --- | --- | --- |
| 01-alias-priority | 别名优先级：高优先级键写在前面 | created 2026-01-15 / updated 2026-02-20 |
| 02-order-independent | 别名解析与 YAML 字段顺序无关 | created 2026-01-15 / updated 2026-02-20 |
| 03-invalid-alias-fallback | 无效高优先级别名不遮挡有效低优先级别名 | created 2026-03-08 / updated 2026-03-09 |
| 04-datetime-forms | ISO 日期、`T` 分隔与空格分隔日期时间均归一为 `YYYY-MM-DD` | created 2026-04-01 / updated 2026-04-02 |
| 05-invalid-dates | 自由文本与非 ISO 格式不进摘要，仅在表格原样展示 | 仅 tags |
| 06-date-fallback | 无 created/updated 时通用 `date` 兜底显示；单个标量 tag | date 2026-06-01 + tags |
| 07-date-suppressed | 存在 created/updated 时通用 `date` 不再显示 | 仅 created 2026-07-01 |
| 08-non-scalar-values | 序列/映射值不进摘要，表格重排为 YAML；行内序列 tags | created 2026-08-01 / updated 2026-08-02 + tags |
| 09-no-frontmatter | 无 frontmatter：摘要与表格都不渲染 | 无 |
| 10-rich-combination | 综合场景：双时间、块序列 tags、自定义字段与嵌套映射 | created/updated/tags |

## 标题来源优先级

标题按 **frontmatter 标量 `title` → 首个非空 H1（任意位置）→ 文件名 basename（含 `.md`）** 的固定链推导，侧边栏与工具栏共用同一条规则。H1 文本会剥离行内标记、解码 HTML 实体并折叠空白。

| 文件 | 验证点 | 显示标题预期 |
| --- | --- | --- |
| 11-frontmatter-over-h1 | frontmatter 标题压过不同的 H1 | 来自 Frontmatter 的标题 |
| 12-empty-title-ignored | 空字符串 `title` 视为无标题 | 空标题被忽略 |
| 13-nonscalar-title-ignored | 序列/映射 `title` 不成为标题，表格仍展示 | 序列标题被忽略 |
| 14-late-first-h1 | H1 不必在文档开头；H2/H3 不参与 | 后置的首个 H1 |
| 15-multiple-h1 | 空文本 H1 终止查找：不取后续 H1，直接落到文件名兜底 | 15-multiple-h1.md |
| 16-h1-text-normalized | 剥离行内标记、解码实体、折叠空白 | 快速 开始 & 部署 |
| 17-filename-fallback | 无 H1 时文件名兜底（保留扩展名） | 17-filename-fallback.md |
| 18-invalid-frontmatter | 非法 YAML：侧边栏降级列出，打开返回 422 | 侧边栏：非法 Frontmatter 的降级；正文：错误态 |

## 描述（description）

`description` 是文档的一句话描述，随 `/api/files` 下发：悬停侧栏条目时显示在文件名与标题下方，侧栏搜索同时匹配标题、文件名、路径与描述。规则与标题一致——只接受标量字符串并去除首尾空白；序列/映射值不成为描述（仍在 Frontmatter 表格原样展示），缺失或为空时侧栏 tooltip 不显示描述行。

本索引文档的 frontmatter 即携带一个 `description`：启动 `m2h web docs/demos` 后悬停侧栏中的"Frontmatter 演示索引"即可对照验证。
