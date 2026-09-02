---
title: 热力图
description: Vega-Lite 热力图示例，rect mark 配合颜色分级展示二维分类密度。
tags:
  - vega-lite
  - 图表
create_date: 2026-09-02
update_date: 2026-09-02
---

# 热力图

`rect` mark 把两个分类字段的交叉格铺满颜色。数据仍完全内联在 `data.values` 中。

```vega-lite
{
  "data": {
    "values": [
      {"day": "周一", "hour": "上午", "load": 3},
      {"day": "周一", "hour": "下午", "load": 7},
      {"day": "周二", "hour": "上午", "load": 4},
      {"day": "周二", "hour": "下午", "load": 9},
      {"day": "周三", "hour": "上午", "load": 5},
      {"day": "周三", "hour": "下午", "load": 6},
      {"day": "周四", "hour": "上午", "load": 8},
      {"day": "周四", "hour": "下午", "load": 4},
      {"day": "周五", "hour": "上午", "load": 6},
      {"day": "周五", "hour": "下午", "load": 2}
    ]
  },
  "mark": "rect",
  "encoding": {
    "x": {"field": "day", "type": "nominal", "title": null},
    "y": {"field": "hour", "type": "nominal", "title": null},
    "color": {"field": "load", "type": "quantitative", "scale": {"scheme": "blues"}, "title": "负载"}
  }
}
```

## 聚合热力图

`aggregate` 让每格自动聚合多行数据，这里对重复的行列组合求平均：

```vega-lite
{
  "data": {
    "values": [
      {"task": "A", "week": 1, "hours": 3},
      {"task": "A", "week": 2, "hours": 5},
      {"task": "A", "week": 1, "hours": 1},
      {"task": "B", "week": 1, "hours": 6},
      {"task": "B", "week": 2, "hours": 2},
      {"task": "B", "week": 2, "hours": 4},
      {"task": "C", "week": 1, "hours": 1},
      {"task": "C", "week": 2, "hours": 7}
    ]
  },
  "mark": "rect",
  "encoding": {
    "x": {"field": "week", "type": "nominal", "title": "周"},
    "y": {"field": "task", "type": "nominal", "title": "任务"},
    "color": {"field": "hours", "aggregate": "mean", "type": "quantitative", "title": "平均小时"}
  }
}
```
