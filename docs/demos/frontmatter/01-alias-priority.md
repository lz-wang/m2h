---
title: 别名优先级（高优先级在前）
create_date: 2026-01-15
create_at: 2026-01-10
create_time: 2026-01-05
update_date: 2026-02-20
update_at: 2026-02-15
update_time: 2026-02-10
---

# 别名优先级（高优先级在前）

同时声明三个 `create_*` 与三个 `update_*` 别名，高优先级键写在 YAML 前面。

预期表现：

- 工具栏摘要显示 **created 2026-01-15**（`create_date` 优先级最高，压过 `create_at` 与 `create_time`）。
- 工具栏摘要显示 **updated 2026-02-20**（`update_date` 同理）。
- Frontmatter 表格按原样列出全部 6 个键，未被选中的别名值不做删除或改写。
