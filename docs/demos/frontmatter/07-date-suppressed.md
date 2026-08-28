---
title: date 让位于 created
date: 2026-07-15
create_at: 2026-07-01
---

# date 让位于 created

同时存在通用 `date` 与 `create_at`，验证 `date` 只在两者都缺失时才兜底。

预期表现：

- 摘要仅显示 **created 2026-07-01**，**不显示** `date: 2026-07-15`。
- 表格中 `date` 条目仍原样可见。
