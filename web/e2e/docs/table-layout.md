---
title: 表格布局回归文档
description: 验证表头单行显示、正文换行、横向滚动和排序。
tags:
  - 测试
  - 表格
create_date: 2026-11-25
update_date: 2026-11-25
---

# 表格布局回归文档

| 字段 | 必填 | Default configuration value | `options fallback value` | 说明 |
| --- | --- | --- | --- | --- |
| beta | 是 | 20 | enabled | 连接失败时，客户端会在指定的间隔后重试，并在超过重试次数后返回错误。 |
| alpha | 是 | 10 | disabled | 为空时，客户端会使用启动参数提供的默认值。 |

<table class="custom-layout" style="width: 130px">
  <thead>
    <tr><th>Custom table heading</th></tr>
  </thead>
  <tbody>
    <tr><td>Value</td></tr>
  </tbody>
</table>
