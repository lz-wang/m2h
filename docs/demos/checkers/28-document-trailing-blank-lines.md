---
title: 文档结尾空白行数量
description: 演示 document.trailing-blank-lines 的判定与边界。
tags: [文档检查]
create_date: 2026-09-10
update_date: 2026-09-10
---

# 文档结尾空白行数量

检查规则：`document.trailing-blank-lines`

文档必须以正文后的恰好一整行空白行结束，即 `正文\n\n`；CRLF 对应 `正文\r\n\r\n`。
只有一个行末换行、没有换行、多于一行空白行或末尾空白行未结束均告警。
只含空格/Tab 的行也算空白行；检查原始文件，Frontmatter 语法错误不会跳过本规则。
诊断定位在文件最后一行（末行无换行时定位至行尾）。
本演示故意只保留一个行末换行，因此报告本规则。

预期输出：

`23:1: warning [document.trailing-blank-lines]: document must end with exactly one blank line (found 0; final newline: true)`

[返回检查规则演示索引](00-index.md)
