# Vega-Lite 基础回归文档

这份文档用于真实浏览器环境下的 Vega-Lite 统计图表基础回归测试。`vega-lite` 围栏代码块中的 JSON spec 由浏览器按需加载 vega、vega-lite、vega-embed 三个运行时后编译渲染为 SVG；下面覆盖条形图、折线图、散点图、分层图、tooltip 交互与 `vegalite` 别名，任何一环断裂都会让图表退化为源码文本。

## 条形图

```vega-lite
{
  "data": {
    "values": [
      {"month": "Jan", "revenue": 120},
      {"month": "Feb", "revenue": 180},
      {"month": "Mar", "revenue": 150}
    ]
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "month", "type": "nominal"},
    "y": {"field": "revenue", "type": "quantitative"}
  }
}
```

这是条形图之后的段落。渲染成功后源码文本不应再出现在正文中，放大按钮随 SVG 出现。

## 折线图

```vega-lite
{
  "data": {
    "values": [
      {"day": 1, "temp": 12},
      {"day": 2, "temp": 15},
      {"day": 3, "temp": 13},
      {"day": 4, "temp": 17}
    ]
  },
  "mark": "line",
  "encoding": {
    "x": {"field": "day", "type": "quantitative"},
    "y": {"field": "temp", "type": "quantitative"}
  }
}
```

## 散点图

```vega-lite
{
  "data": {
    "values": [
      {"x": 1, "y": 2},
      {"x": 2, "y": 4},
      {"x": 3, "y": 3},
      {"x": 4, "y": 6}
    ]
  },
  "mark": "point",
  "encoding": {
    "x": {"field": "x", "type": "quantitative"},
    "y": {"field": "y", "type": "quantitative"}
  }
}
```

## 分层图

```vega-lite
{
  "data": {
    "values": [
      {"month": "Jan", "revenue": 120, "cost": 80},
      {"month": "Feb", "revenue": 180, "cost": 110},
      {"month": "Mar", "revenue": 150, "cost": 95}
    ]
  },
  "layer": [
    {
      "mark": "bar",
      "encoding": {
        "x": {"field": "month", "type": "nominal"},
        "y": {"field": "revenue", "type": "quantitative"}
      }
    },
    {
      "mark": "line",
      "encoding": {
        "x": {"field": "month", "type": "nominal"},
        "y": {"field": "cost", "type": "quantitative"}
      }
    }
  ]
}
```

## tooltip 交互

悬停下面的散点应显示 vega-tooltip 浮层。

```vega-lite
{
  "data": {
    "values": [
      {"x": 1, "y": 2},
      {"x": 2, "y": 5},
      {"x": 3, "y": 4}
    ]
  },
  "mark": "point",
  "encoding": {
    "x": {"field": "x", "type": "quantitative"},
    "y": {"field": "y", "type": "quantitative"}
  }
}
```

## 别名 vegalite

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

这是文档最后一段，用于保证断言运行时页面布局稳定。
