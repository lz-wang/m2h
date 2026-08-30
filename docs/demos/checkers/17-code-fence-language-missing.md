---
title: 代码块没有语言
---

# 代码块没有语言

下面的 fenced code 没有写语言，渲染器无法做语法高亮：

```
func main() { println("hello") }
```

预期表现：

- `m2h check docs/demos/checkers/17-code-fence-language-missing.md` 报告
  `9:1: warning [code-fence.language-missing]: fenced code block has no language`
- 只检查 fenced code；缩进四个空位的 indented code 不适用本规则
