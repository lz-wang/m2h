---
title: 图片没有 alt 文本
description: 演示图片缺少可访问替代文本时的警告。
tags:
  - 文档检查
  - 可访问性
create_date: 2026-08-30
update_date: 2026-08-30
checker: image.alt-empty
---

# 图片没有 alt 文本

检查规则：`image.alt-empty`

下面这张图存在且可渲染，但没有 alt 文本——读屏用户得不到任何信息：

![](assets/logo.png)

预期表现：

- `m2h check docs/demos/checkers/13-image-alt-empty.md` 报告
  `18:5: warning [image.alt-empty]: image has no alt text`
- 装饰性图片使用空 alt 是合法做法，因此本规则是 warning 而不是 error

[返回检查规则演示索引](00-index.md)
