---
title: 未解决的合并冲突
description: 演示 Markdown 文件残留 Git 冲突标记时的错误。
tags:
  - 文档检查
  - Git
create_date: 2026-11-14
update_date: 2026-11-14
checker: document.merge-conflict
---

# 未解决的合并冲突

检查规则：`document.merge-conflict`

直接检查文件源码中的 Git 冲突标记，不依赖 Git 仓库状态或外部命令。
下面的冲突块是 `<<<<<<<` 打开、`>>>>>>>` 关闭的残留标记，文档会把
冲突两侧一起发布成正文。本演示的 ours 一侧留空，因此不会顺带触发
`document.multiple-h1`（非空的 ours 正文紧邻 `=======` 会被 Markdown
读成 Setext 标题）。

预期表现：

- `m2h check docs/demos/checkers/31-document-merge-conflict.md` 报告
  `36:1: error [document.merge-conflict]: unresolved Git merge conflict marker`
- 完整冲突块只报告一次，定位在 `<<<<<<<` 开始行；只有 `<<<<<<<` 的
  未闭合残留同样报告
- 单独出现的 `=======` 是 Setext 标题语法、单独的 `>>>>>>>` 是普通
  文本，均不报告；代码块、行内代码、HTML 注释与数学公式里的冲突
  示例是讲解材料，不报告
- 冲突出现在 Frontmatter 内并破坏 YAML 时，同时报告
  `frontmatter.invalid` 与本规则

[返回检查规则演示索引](00-index.md)

<<<<<<< HEAD

=======
合并进来的一侧
>>>>>>> feature
