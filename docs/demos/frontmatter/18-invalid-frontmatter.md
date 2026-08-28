---
title: [未闭合的序列
---

# 非法 Frontmatter 的降级

frontmatter 块的 YAML 非法（流序列缺少右括号），解析必然失败。

预期表现：

- 侧边栏**仍然列出本文件**，标题降级为正文推导（H1 **非法 Frontmatter 的降级**）：单个文件的坏 frontmatter 不拖垮整个文件列表。
- 在浏览器中打开本文档时，文档接口返回 422 与 `invalid frontmatter` 错误，页面呈现错误态。
- `/raw/18-invalid-frontmatter.md` 仍可访问原始 Markdown 源文本。
