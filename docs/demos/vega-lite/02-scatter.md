---
title: 散点图与颜色编码
description: Vega-Lite 散点图示例，演示 point mark 与 color 通道的字段编码及图例。
tags:
  - vega-lite
  - 图表
create_date: 2026-09-02
update_date: 2026-09-02
---

# 散点图与颜色编码

`point` mark 把每行数据画成一个点；`color` 通道把分类字段编码为颜色并自动生成图例。作者选择的颜色语义不会随主题切换改变。

```vega-lite
{
  "data": {
    "values": [
      {"x": 1, "y": 2, "group": "one"},
      {"x": 2, "y": 4, "group": "one"},
      {"x": 3, "y": 3, "group": "one"},
      {"x": 4, "y": 6, "group": "one"},
      {"x": 1.5, "y": 3, "group": "two"},
      {"x": 2.5, "y": 5, "group": "two"},
      {"x": 3.5, "y": 4, "group": "two"}
    ]
  },
  "mark": {"type": "point", "filled": true, "size": 80},
  "encoding": {
    "x": {"field": "x", "type": "quantitative"},
    "y": {"field": "y", "type": "quantitative"},
    "color": {"field": "group", "type": "nominal"}
  }
}
```

## 连续颜色编码

把 `type` 设为 `quantitative` 时颜色通道编码为连续色阶，图例随之变为渐变条。

```vega-lite
{
  "data": {
    "values": [
      {"x": 1, "y": 2, "score": 30},
      {"x": 2, "y": 4, "score": 55},
      {"x": 3, "y": 3, "score": 42},
      {"x": 4, "y": 6, "score": 88},
      {"x": 5, "y": 1, "score": 15},
      {"x": 1.5, "y": 5, "score": 67}
    ]
  },
  "mark": {"type": "point", "filled": true, "size": 120},
  "encoding": {
    "x": {"field": "x", "type": "quantitative"},
    "y": {"field": "y", "type": "quantitative"},
    "color": {"field": "score", "type": "quantitative", "scale": {"scheme": "viridis"}}
  }
}
```
