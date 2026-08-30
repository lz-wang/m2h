---
title: 没有信息量的链接文本
---

# 没有信息量的链接文本

[click here](https://example.com) 和 [点击这里](https://example.com) 都没有
告诉读者链接指向什么，读屏用户按链接列表导航时无从选择。

预期表现：

- 默认不检查：`m2h check docs/demos/checkers/23-link-text-nondescriptive.md`
  无诊断
- `--enable link.text-nondescriptive` 后报告两条
  `7:2` 与 `7:40: warning [link.text-nondescriptive]: link text "click here" is not descriptive`
- 只做中英文小词表精确匹配：`点击这里查看 API 文档` 包含实际信息，不报；
  图片 alt 与 raw HTML 不在范围内
