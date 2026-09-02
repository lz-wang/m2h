---
title: 直方图
description: Vega-Lite 直方图示例，演示内置 bin 分箱与计数聚合。
tags:
  - vega-lite
  - 图表
create_date: 2026-09-02
update_date: 2026-09-02
---

# 直方图

直方图不需要预先分箱：把字段的 `bin` 打开，`y` 轴使用 `count` 聚合，Vega-Lite 在浏览器内完成分箱与计数。

```vega-lite
{
  "data": {
    "values": [
      {"wait": 4}, {"wait": 7}, {"wait": 9}, {"wait": 12}, {"wait": 15},
      {"wait": 18}, {"wait": 21}, {"wait": 23}, {"wait": 26}, {"wait": 28},
      {"wait": 31}, {"wait": 34}, {"wait": 37}, {"wait": 41}, {"wait": 45},
      {"wait": 48}, {"wait": 52}, {"wait": 57}, {"wait": 63}, {"wait": 71}
    ]
  },
  "mark": "bar",
  "encoding": {
    "x": {"bin": true, "field": "wait", "type": "quantitative", "title": "等待时间"},
    "y": {"aggregate": "count", "type": "quantitative", "title": "数量"}
  }
}
```

## 按类别着色的直方图

`color` 通道叠加分类维度，分箱内按类别堆叠：

```vega-lite
{
  "data": {
    "values": [
      {"wait": 5, "lane": "快"}, {"wait": 8, "lane": "快"}, {"wait": 11, "lane": "快"},
      {"wait": 14, "lane": "快"}, {"wait": 6, "lane": "慢"}, {"wait": 19, "lane": "慢"},
      {"wait": 27, "lane": "慢"}, {"wait": 33, "lane": "慢"}, {"wait": 9, "lane": "慢"},
      {"wait": 12, "lane": "快"}, {"wait": 21, "lane": "快"}, {"wait": 30, "lane": "慢"}
    ]
  },
  "mark": "bar",
  "encoding": {
    "x": {"bin": true, "field": "wait", "type": "quantitative", "title": "等待时间"},
    "y": {"aggregate": "count", "type": "quantitative", "title": "数量"},
    "color": {"field": "lane", "type": "nominal", "title": "通道"}
  }
}
```
