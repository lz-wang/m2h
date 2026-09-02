# Vega-Lite 安全回归文档

这份文档用于真实浏览器环境下的 Vega-Lite 网络隔离回归测试。宿主提供的 loader 拒绝一切外部资源加载：`data.url` 指向外部地址的 spec 不应发出任何网络请求（依赖 loader 拒绝而不是 CSP 拦截），同一文档中的正常自包含图表应不受影响地渲染。

## 外部 data.url

```vega-lite
{
  "data": {
    "url": "https://example.invalid/data.csv"
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "a", "type": "nominal"}
  }
}
```

这张图表应渲染失败并保留源码文本，浏览器不应对 example.invalid 发起请求。

## 正常图表

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

上面的失败不应影响这张自包含图表正常渲染。
