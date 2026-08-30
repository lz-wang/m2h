---
title: 标题层级向下跳跃
---

# API 参考

下面直接从 H1 跳到 H4，目录树里中间凭空缺了两层：

#### 参数列表

预期表现：

- `m2h check docs/demos/checkers/15-heading-level-skip.md` 报告
  `9:1: warning [heading.level-skip]: heading level jumps from H1 to H4`
- 向上跳任意级是合法的章节收尾；文档第一条标题即使是 H3 也不报
