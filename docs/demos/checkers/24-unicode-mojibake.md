---
title: 疑似错误编码文本
description: 演示文本出现常见错误编码签名时的警告。
tags:
  - 文档检查
  - 文本质量
create_date: 2026-08-30
update_date: 2026-08-30
checker: unicode.mojibake
---

# 疑似错误编码文本

检查规则：`unicode.mojibake`

下面这行文字是"咖啡馆"的 UTF-8 字节被按 Latin-1 误解码后又重新编码的
典型产物（正确写法应为"一杯 Café"）：

一杯 CafÃ©，编码出错的拿铁。

引号类签名同理：Itâ€™s 出自弯引号 `’`。

预期表现：

- 默认不检查：`m2h check docs/demos/checkers/24-unicode-mojibake.md` 无诊断
- `--enable unicode.mojibake` 后按签名逐处报告
  `19:11` 与 `21:27: warning [unicode.mojibake]: suspicious mojibake "Ã©"`（另一处为 `"â€™"`）
- 只用多字符签名判定：单独一个 `Ã` 或 `Â` 是多门语言的合法字母，不报
  （行内代码里的内容也永不参与判定）

[返回检查规则演示索引](00-index.md)
