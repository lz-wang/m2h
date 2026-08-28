---
title: 无效别名不遮挡有效值
create_date: 待定
create_at: 2026-03-08T09:00:00+08:00
update_date: 2026-03-09
update_time: 2026-03-01
---

# 无效别名不遮挡有效值

最高优先级的 `create_date` 是自由文本"待定"，无法归一为 ISO 日期；`update_date` 有效但被更低优先级的 `update_time` 抢先出现。

预期表现：

- 摘要显示 **created 2026-03-08**：无效的 `create_date` 不遮挡有效的 `create_at`（带时区的日期时间归一为日）。
- 摘要显示 **updated 2026-03-09**：`update_date` 压过后出现的低优先级 `update_time`。
- 表格中 `create_date: 待定` 原样可见。
