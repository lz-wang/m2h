---
title: 条形图、折线图与分层图
description: Vega-Lite 条形图、折线图与分层图基础示例，覆盖坐标轴标题、字段编码与 vegalite 别名 fence。
tags:
  - vega-lite
  - 图表
create_date: 2026-09-02
update_date: 2026-09-02
---

# 条形图、折线图与分层图

条形图是 Vega-Lite 最小可用示例：内联 `data.values` 提供数据，`mark` 声明图形，`encoding` 把字段映射到坐标轴。

```vega-lite
{
  "data": {
    "values": [
      {"month": "Jan", "revenue": 120},
      {"month": "Feb", "revenue": 180},
      {"month": "Mar", "revenue": 150},
      {"month": "Apr", "revenue": 210}
    ]
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "month", "type": "nominal", "title": "月份"},
    "y": {"field": "revenue", "type": "quantitative", "title": "收入"}
  }
}
```

## 折线图

把 `mark` 换成 `line` 即得到折线图；时间或数值字段放到 `x` 轴。

```vega-lite
{
  "data": {
    "values": [
      {"day": 1, "temp": 12},
      {"day": 2, "temp": 15},
      {"day": 3, "temp": 13},
      {"day": 4, "temp": 17},
      {"day": 5, "temp": 14}
    ]
  },
  "mark": {"type": "line", "point": true},
  "encoding": {
    "x": {"field": "day", "type": "quantitative", "title": "天"},
    "y": {"field": "temp", "type": "quantitative", "title": "温度"}
  }
}
```

## 分层图

`layer` 把多个 mark 叠加在同一坐标系中，共用同一份底层数据。

```vega-lite
{
  "data": {
    "values": [
      {"month": "Jan", "revenue": 120, "cost": 80},
      {"month": "Feb", "revenue": 180, "cost": 110},
      {"month": "Mar", "revenue": 150, "cost": 95},
      {"month": "Apr", "revenue": 210, "cost": 130}
    ]
  },
  "layer": [
    {
      "mark": "bar",
      "encoding": {
        "x": {"field": "month", "type": "nominal"},
        "y": {"field": "revenue", "type": "quantitative", "title": "收入"}
      }
    },
    {
      "mark": {"type": "line", "color": "#d62728"},
      "encoding": {
        "x": {"field": "month", "type": "nominal"},
        "y": {"field": "cost", "type": "quantitative", "title": "成本"}
      }
    }
  ]
}
```

## 别名 fence

` ```vegalite ` 与 ` ```vega-lite ` 完全等价，下面的图表用别名声明：

```vegalite
{
  "data": {
    "values": [
      {"kind": "A", "count": 28},
      {"kind": "B", "count": 55}
    ]
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "kind", "type": "nominal"},
    "y": {"field": "count", "type": "quantitative"}
  }
}
```
