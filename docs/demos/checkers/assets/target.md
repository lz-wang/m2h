---
title: 相邻目标文档
---

# 相邻目标文档

这是一个自身完全干净的演示辅助文档：它存在的意义是**被别的演示文件引用**。

在目录模式（`m2h check docs/demos/checkers`）下它是被服务的目标，引用它的
`05-markdown-target-not-served.md` 不会产生诊断；在单文件模式下它不在服务
范围内，因此触发 `markdown-target.not-served`。
