---
title: 非法日期不进摘要
create_date: March 8th
create_at: 08/03/2026
update_at: 2026-3-8
tags: [测试, 边界]
---

# 非法日期不进摘要

三个日期别名全部是非法格式：自由文本、`DD/MM/YYYY`、以及非补零的 `2026-3-8`。

预期表现：

- 摘要**不显示**任何 created/updated（也没有通用 `date` 兜底），仅显示 tags。
- 表格中原样保留 `March 8th`、`08/03/2026`、`2026-3-8`，不猜测、不转换。
