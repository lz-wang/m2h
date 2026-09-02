---
title: 交互选择与 tooltip
description: Vega-Lite 交互示例，演示 point 选择高亮与默认 tooltip 悬停提示。
tags:
  - vega-lite
  - 图表
create_date: 2026-09-02
update_date: 2026-09-02
---

# 交互选择与 tooltip

Web 文档服务与导出 HTML 中的图表保持完整交互能力：悬停任意数据 mark 会显示 tooltip，`params` 定义的选择高亮在浏览器中原生工作。

## tooltip

悬停下面的柱子查看 tooltip（Vega-Embed 内置，无需额外配置）：

```vega-lite
{
  "data": {
    "values": [
      {"city": "北京", "population": 2189},
      {"city": "上海", "population": 2487},
      {"city": "广州", "population": 1874},
      {"city": "深圳", "population": 1768}
    ]
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "city", "type": "nominal", "title": "城市"},
    "y": {"field": "population", "type": "quantitative", "title": "人口（万）"}
  }
}
```

## 选择高亮

点击或框选下面的点，被选中的数据保持高亮，其余淡化：

```vega-lite
{
  "data": {
    "values": [
      {"x": 1, "y": 2, "group": "one"}, {"x": 2, "y": 4, "group": "one"},
      {"x": 3, "y": 3, "group": "one"}, {"x": 4, "y": 6, "group": "one"},
      {"x": 1.5, "y": 3, "group": "two"}, {"x": 2.5, "y": 5, "group": "two"},
      {"x": 3.5, "y": 4, "group": "two"}, {"x": 5, "y": 7, "group": "two"}
    ]
  },
  "params": [
    {"name": "sel", "select": {"type": "point", "toggle": false}}
  ],
  "mark": {"type": "point", "filled": true, "size": 100},
  "encoding": {
    "x": {"field": "x", "type": "quantitative"},
    "y": {"field": "y", "type": "quantitative"},
    "color": {
      "field": "group",
      "type": "nominal",
      "condition": {"param": "sel", "empty": false},
      "value": "lightgray"
    }
  }
}
```
