---
title: 检查规则演示索引
description: m2h check 的 25 条检查规则、触发示例与预期诊断。
tags:
  - 文档检查
  - 演示索引
create_date: 2026-08-30
update_date: 2026-08-30
---

# 检查规则演示索引

本目录为 `m2h check` 的全部 25 条检查规则各提供一个最小触发示例。每个演示
文件只触发自己的规则（个别文件会顺带演示规则间的独立性），文件内的
"预期表现"一节写明命令与逐字输出。

一次性看全部默认开启的规则（12 error + 9 warning）：

```bash
m2h check docs/demos/checkers
```

逐个隔离验证时用单文件模式（05 号演示只在单文件模式下触发；其余文件在两种
模式下诊断相同）：

```bash
m2h check docs/demos/checkers/15-heading-level-skip.md
```

四条 opt-in 规则默认不运行，需 `--enable` 显式开启：

```bash
m2h check docs/demos/checkers --enable section.empty,link.text-nondescriptive,unicode.mojibake,unicode.invisible-character
```

## 选择规则

`--enable` 在默认规则之上追加规则，`--disable` 从中移除规则；两者都支持
逗号分隔多个规则，`all` 代表全部规则，且 `--disable` 始终优先：

```bash
m2h check docs --enable section.empty,unicode.mojibake
m2h check docs --disable image.alt-empty
m2h check docs --enable all --disable image.alt-empty
```

因此，`--disable all --enable <规则>` 的结果是一条规则都不运行，而不是只运行
指定规则。未知规则名会在读取任何文件之前以
`Error: unknown check rule "foo.bar"` 失败。

`assets/` 是演示辅助目录：`target.md` 自身完全干净、仅被 05 号引用，
`logo.png` 是真实存在的 1×1 图片。

## 语义边界

- 本地引用中，`images/logo.png` 相对当前文档，`/images/logo.png` 相对当前输入
  root（多 root 模式下为引用所在的 root），而 `//cdn.example.com/logo.png`
  仍是协议相对网络 URL。WebUI 路由与检查器共享这套分类和越界边界。
- 引用、脚注和反转链接扫描以真实 Goldmark AST 为边界：行内/块级代码、HTML
  block、HTML comment 与 inline raw HTML tag token 不参与；inline tag 之间的
  文本仍是普通 Markdown，会按渲染器相同语义检查。
- 已被解析器接受的 inline link destination/title 是链接语法而不是嵌套正文，
  其中看似 reference、脚注或反转链接的括号组合不会产生诊断。
- `section.empty` 的“内容”指实际渲染节点；thematic break 会生成 `<hr>`，因此
  计为内容，reference definition、HTML comment 与空白不计。
- `unicode.*` 检查源文件文本质量，仍扫描 raw HTML，仅代码区域豁免。

## error 规则（默认开启）

| 演示 | 规则 | 触发场景 |
| --- | --- | --- |
| [01-frontmatter-invalid](01-frontmatter-invalid.md) | `frontmatter.invalid` | Frontmatter YAML 无法解析（未闭合序列），文档完全不可渲染 |
| [02-local-target-missing](02-local-target-missing.md) | `local-target.missing` | 图片引用的本地文件不存在 |
| [03-local-target-not-regular](03-local-target-not-regular.md) | `local-target.not-regular` | 链接指向目录而不是普通文件 |
| [04-local-target-outside-root](04-local-target-outside-root.md) | `local-target.outside-root` | 连续 `../` 越出文档根目录 |
| [05-markdown-target-not-served](05-markdown-target-not-served.md) | `markdown-target.not-served` | 单文件模式下引用相邻文档（文件存在但不被服务） |
| [06-anchor-missing](06-anchor-missing.md) | `anchor.missing` | 页内锚点指向不存在的标题 |
| [07-reference-undefined](07-reference-undefined.md) | `reference.undefined` | `[text][label]` 引用了不存在的 reference 定义 |
| [08-footnote-undefined](08-footnote-undefined.md) | `footnote.undefined` | `[^label]` 脚注标记没有定义 |
| [09-footnote-empty](09-footnote-empty.md) | `footnote.empty` | 脚注定义没有任何内容 |
| [10-table-column-mismatch](10-table-column-mismatch.md) | `table.column-mismatch` | 表格数据行列数多于分隔行（渲染时被截断） |
| [11-html-comment-unclosed](11-html-comment-unclosed.md) | `html.comment-unclosed` | `<!--` 未闭合，其后内容整体渲染为注释 |
| [12-link-reversed](12-link-reversed.md) | `link.reversed` | 高置信度反转链接写法 `(text)[url]` |

## warning 规则（默认开启）

| 演示 | 规则 | 触发场景 |
| --- | --- | --- |
| [13-image-alt-empty](13-image-alt-empty.md) | `image.alt-empty` | 图片没有 alt 文本 |
| [14-document-multiple-h1](14-document-multiple-h1.md) | `document.multiple-h1` | 一个文档包含多个 H1 |
| [15-heading-level-skip](15-heading-level-skip.md) | `heading.level-skip` | 标题层级从 H1 直接跳到 H4 |
| [16-heading-duplicate](16-heading-duplicate.md) | `heading.duplicate` | 同一父章节下重复的"安装"标题 |
| [17-code-fence-language-missing](17-code-fence-language-missing.md) | `code-fence.language-missing` | fenced code 未指定语言 |
| [18-footnote-unused](18-footnote-unused.md) | `footnote.unused` | 有内容但从未被引用的脚注 |
| [19-reference-unused](19-reference-unused.md) | `reference.unused` | 定义了但从未使用的 reference |
| [20-frontmatter-date-invalid](20-frontmatter-date-invalid.md) | `frontmatter.date-invalid` | Frontmatter 日期字段写了不存在的日期（2 月 30 日） |
| [21-link-empty-destination](21-link-empty-destination.md) | `link.empty-destination` | 链接 destination 为空（顺带演示与 `image.alt-empty` 独立报告） |

## warning 规则（默认关闭，`--enable` 开启）

| 演示 | 规则 | 触发场景 |
| --- | --- | --- |
| [22-section-empty](22-section-empty.md) | `section.empty` | 标题与下一标题之间没有任何渲染内容（`<hr>` 计为内容） |
| [23-link-text-nondescriptive](23-link-text-nondescriptive.md) | `link.text-nondescriptive` | 链接文本为 `click here` / `点击这里` 等无信息短语 |
| [24-unicode-mojibake](24-unicode-mojibake.md) | `unicode.mojibake` | `CafÃ©`、`Itâ€™s` 等多字符错误编码签名 |
| [25-unicode-invisible-character](25-unicode-invisible-character.md) | `unicode.invisible-character` | 行首零宽空格（U+200B）等可疑不可见字符 |

## 与规则清单的对应

本索引集中维护规则名称、等级、默认开关和触发场景；各演示文件的"预期表现"
按当前实现逐字核对，并补充对应规则的行为边界。输出契约为
`path:line:column: severity [rule]: message`，发现 error（或 `--strict` 下存在
warning）时退出码为 1。交互式终端只着色 severity 和总结结果：error 为红色、
warning 为黄色、全部通过为绿色；重定向、管道、`NO_COLOR` 与 JSON 输出无颜色。
文本诊断失败仍通过退出码表达，统计摘要是最后一行，不再追加重复的失败描述。
