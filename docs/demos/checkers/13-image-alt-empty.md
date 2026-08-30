---
title: 图片没有 alt 文本
---

# 图片没有 alt 文本

下面这张图存在且可渲染，但没有 alt 文本——读屏用户得不到任何信息：

![](assets/logo.png)

预期表现：

- `m2h check docs/demos/checkers/13-image-alt-empty.md` 报告
  `9:5: warning [image.alt-empty]: image has no alt text`
- 装饰性图片使用空 alt 是合法做法，因此本规则是 warning 而不是 error
