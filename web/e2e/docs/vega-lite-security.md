# Vega-Lite 安全回归文档

这份文档用于真实浏览器环境下的 Vega-Lite 网络隔离回归测试。宿主提供的 loader 拒绝一切外部资源加载：`data.url` 指向外部地址的 spec 不应发出任何网络请求（依赖宿主策略拒绝而不是 CSP 拦截），同一文档中的正常自包含图表应不受影响地渲染。

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

这张图表应在嵌入前被判为不支持的外部数据源：保留源码文本、没有 SVG、没有放大按钮、仅控制台警告，浏览器不应对 example.invalid 发起请求。

## 嵌套 spec 中的 data.url

分层组合的子图表同样不允许引用外部数据，宿主的判定递归覆盖组合位置。

```vega-lite
{
  "layer": [
    {
      "data": {
        "url": "https://example.invalid/nested.json"
      },
      "mark": "point",
      "encoding": {
        "x": {"field": "a", "type": "quantitative"}
      }
    }
  ]
}
```

这张图表应与上一张同样失败并保留源码文本。

## 外部图片 mark

```vega-lite
{
  "data": {
    "values": [
      {"x": 1, "y": 1}
    ]
  },
  "mark": {
    "type": "image",
    "clip": true
  },
  "encoding": {
    "x": {"field": "x", "type": "quantitative"},
    "y": {"field": "y", "type": "quantitative"},
    "url": {"value": "https://example.invalid/badge.png"}
  }
}
```

图片资源同样不发出外部请求：loader 在解析前拒绝地址，图片 mark 不加载，图表本身仍渲染出坐标系，且不进入放大预览或源码回退以外的状态。

## 跨域超链接图表

```vega-lite
{
  "data": {
    "values": [
      {"site": "docs", "score": 3, "link": "https://example.invalid/linked"}
    ]
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "site", "type": "nominal"},
    "y": {"field": "score", "type": "quantitative"},
    "href": {"field": "link", "type": "nominal"}
  }
}
```

图表生成的跨 origin 链接遵循阅读器的链接策略：新标签页打开并带 `noopener`，而不是在当前标签页导航离开。

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
