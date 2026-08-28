---
title: 序列与映射值不进摘要
create_date:
  - 2026-01-01
  - 2026-02-01
create_time: 2026-08-01
update_date:
  editor: alice
  when: 2026-08-02T10:00:00
update_time: 2026-08-02
tags: [YAML, 序列值]
---

# 序列与映射值不进摘要

`create_date` 写成块序列、`update_date` 写成映射，只有最低优先级的标量别名有效。

预期表现：

- 摘要显示 **created 2026-08-01**（来自 `create_time`）与 **updated 2026-08-02**（来自 `update_time`）：序列与映射无法归一为单个日期，也不遮挡有效别名。
- 表格将序列与映射重排为可读 YAML 展示（多行值在单元格内保持可读）。
- 行内序列 tags 显示为 `YAML · 序列值`。
