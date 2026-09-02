# Vega-Lite 失败隔离回归文档

这份文档用于真实浏览器环境下的 Vega-Lite 失败隔离回归测试。无效的 JSON spec 与有效 spec 共存于同一文档时，无效图表保留源码文本、不提供放大按钮，有效图表不受影响地正常渲染。

## 无效 JSON

```vega-lite
{ not valid json
```

上面的图表应保持为源码文本，不出现 SVG 与放大按钮。

## 编译失败的 spec

```vega-lite
{
  "mark": "bar",
  "encoding": {
    "x": {"field": "month", "type": "not-a-valid-type"}
  }
}
```

## 有效图表

```vega-lite
{
  "data": {
    "values": [
      {"month": "Jan", "revenue": 120},
      {"month": "Feb", "revenue": 180}
    ]
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "month", "type": "nominal"},
    "y": {"field": "revenue", "type": "quantitative"}
  }
}
```

前面的失败不应影响这张图表正常渲染为 SVG。
