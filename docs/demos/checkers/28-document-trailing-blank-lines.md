---
title: 文档结尾空白行数量
description: 演示 document.trailing-blank-lines 的判定与边界。
tags: [文档检查]
create_date: 2026-09-10
update_date: 2026-09-10
---

# 文档结尾空白行数量

检查规则：`document.trailing-blank-lines`

文档应以恰好一个换行符结束，即 `正文\n`；CRLF 对应 `正文\r\n`。
编辑器因此显示最后一行为空，已经符合规则，不需要再按一次回车。
没有结束换行、额外空白行（`正文\n\n`）或末尾未结束的空白行均告警。
额外的空格/Tab 行也算空白行；正文行末的空格不由本规则检查。
检查原始文件，Frontmatter 语法错误不会跳过本规则。
诊断定位在文件最后一行（末行无换行时定位至行尾）。
规则 ID 保持为 `document.trailing-blank-lines`，已有 `--disable` 配置仍可使用。
本演示故意在末尾多保留一个换行，因此报告本规则。

预期输出：

`27:1: warning [document.trailing-blank-lines]: document must end with a single newline (LF or CRLF), without trailing blank lines`

[返回检查规则演示索引](00-index.md)

