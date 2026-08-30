---
title: 可疑的不可见字符
description: 演示文本中出现可疑不可见字符时的警告。
tags:
  - 文档检查
  - 文本质量
create_date: 2026-08-30
update_date: 2026-08-30
checker: unicode.invisible-character
---

# 可疑的不可见字符

检查规则：`unicode.invisible-character`

下面这一行的行首藏着一个零宽空格（U+200B），肉眼看不见，却会影响搜索、
复制与 diff：

​这一行的第一个字符是零宽空格。

预期表现：

- 默认不检查：`m2h check docs/demos/checkers/25-unicode-invisible-character.md`
  无诊断
- `--enable unicode.invisible-character` 后报告
  `19:1: warning [unicode.invisible-character]: suspicious invisible character U+200B ZERO WIDTH SPACE`
- 保守触发：仅行首/行尾、邻接空白、连续出现或 bidi 控制字符报告；
  emoji 依赖的 ZWJ（U+200D）与 variation selector（U+FE0F）永不报告

[返回检查规则演示索引](00-index.md)
