---
title: 表格行列数不一致
---

# 表格行列数不一致

下面的表格分隔行声明了两列，第一条数据行规规矩矩，第二条却写了三列——
渲染时第三列会被悄悄截掉：

| 名称 | 说明 |
| --- | --- |
| m2h | Markdown 工具 |
| 多出来的列 | 第二列 | 第三列 |

预期表现：

- `m2h check docs/demos/checkers/10-table-column-mismatch.md` 报告
  `13:1: error [table.column-mismatch]: table row has 3 columns; expected 2`
- 列数不足的行会被悄悄补空 cell；表头多于分隔线时整表被拒绝、什么都不渲染，
  同样报本规则
