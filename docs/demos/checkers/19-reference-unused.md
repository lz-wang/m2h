---
title: 从未被使用的 reference 定义
---

# 从未被使用的 reference 定义

两个定义中只有一个被正文使用：

[官网][site]

[site]: https://example.com
[unused]: https://example.com/orphan

预期表现：

- `m2h check docs/demos/checkers/19-reference-unused.md` 报告
  `12:1: warning [reference.unused]: reference definition "unused" is never used`
- 标签按渲染器同一归一化比较：`[文字][SITE]` 同样算使用 `[site]`
