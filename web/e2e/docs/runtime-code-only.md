---
title: 纯代码运行时回归
description: 验证代码中的图表格式与公式标记不会触发富内容运行时
tags:
  - regression
create_date: 2026-10-14
update_date: 2026-10-14
---

# 纯代码运行时回归

```vega-lite
{"$schema":"https://vega.github.io/schema/vega-lite/v6.json","mark":"bar"}
```

```mermaid
zenuml
  Alice->Bob: Hello
```

行内代码 `$E=mc^2$` 同样保持源码。
